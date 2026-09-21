// seed populates a local fitter database with a fake athlete, a spread of
// activities across multiple sport groups, a sample campaign, and a couple
// of clause-based goals — enough to exercise the UI against real data
// without a live Strava account.
//
// Every activity is built as a strava.Activity and pushed through
// internal/ingest.UpsertActivity — the same single write path the webhook,
// backfill, and resync tools use — so this fixture data can never drift
// from what real ingestion produces.
//
// Usage:
//
//	go run ./cmd/seed
//
// DB_PATH (default "fitter.db") selects which SQLite file to seed; point it
// at the same file the server uses (see main.go) to see the fixtures in the
// running app. Re-running against the same DB file updates the same
// activities (upsert is keyed on strava_activity_id) but will insert a
// second copy of the sample campaign and goals, since those don't have an
// analogous natural key here — start from a fresh DB file for a clean seed.
//
// JWT_SECRET (must match the value the server was started with) lets this
// tool mint a ready-to-use session token for the seeded user and print a
// login URL — so the app can be exercised end to end without ever
// authenticating against the real Strava API. FRONTEND_URL (default
// http://localhost:5173) is used only to build that printed URL.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	dbfs "github.com/fitter/svc/db"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/ingest"
	"github.com/fitter/svc/internal/strava"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"

	_ "modernc.org/sqlite"
)

const seedJWTTTL = 30 * 24 * time.Hour

const seedUserID = "seed-user-1"
const seedStravaAthleteID = int64(1000001)

// A second seeded user, joined to the same sample campaign, so multi-member
// leaderboard/timeline behavior (sport-group collisions, per-clause
// labeling of OTHER members' entries) is exercisable without a second real
// Strava account. See seedActivitiesTwo/seedGoalsTwo.
const seedUserTwoID = "seed-user-2"
const seedStravaAthleteTwoID = int64(1000002)

func main() {
	dbPath := envOr("DB_PATH", "fitter.db")
	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		fatal("open database failed", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(dbfs.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		fatal("goose set dialect failed", err)
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		fatal("run migrations failed", err)
	}

	q := db.New(sqlDB)
	ctx := context.Background()

	user := seedUser(ctx, q)
	slog.Info("seeded user", "id", user.ID, "name", user.Name)

	userTwo := seedUserTwo(ctx, q)
	slog.Info("seeded user", "id", userTwo.ID, "name", userTwo.Name)

	activities := seedActivities()
	for _, a := range activities {
		if err := ingest.UpsertActivity(ctx, q, user.ID, a); err != nil {
			fatal("seed activity upsert failed", err)
		}
	}
	slog.Info("seeded activities", "count", len(activities))

	activitiesTwo := seedActivitiesTwo()
	for _, a := range activitiesTwo {
		if err := ingest.UpsertActivity(ctx, q, userTwo.ID, a); err != nil {
			fatal("seed activity upsert failed", err)
		}
	}
	slog.Info("seeded activities", "count", len(activitiesTwo), "user", userTwo.ID)

	campaign := seedCampaign(ctx, q, user)
	slog.Info("seeded campaign", "id", campaign.ID, "name", campaign.Name)

	seedGoals(ctx, q, user, campaign)
	slog.Info("seeded goals")

	joinCampaignTwo(ctx, q, userTwo, campaign)
	seedGoalsTwo(ctx, q, userTwo, campaign)
	slog.Info("seeded goals", "user", userTwo.ID)

	slog.Info("seed complete", "db_path", dbPath)

	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		token, err := auth.Issue([]byte(secret), user.ID, seedJWTTTL)
		if err != nil {
			fatal("issue seed jwt failed", err)
		}
		frontendURL := envOr("FRONTEND_URL", "http://localhost:5173")
		fmt.Println()
		fmt.Println("Open this URL to log in as the seeded user (no Strava needed):")
		fmt.Printf("  %s/auth/callback#token=%s\n", frontendURL, token)
	} else {
		fmt.Println()
		fmt.Println("Set JWT_SECRET (matching the running server) to also get a ready login URL.")
	}
}

// seedUser upserts a single fake athlete. UpsertUserByStravaID is keyed on
// strava_athlete_id, so re-running the seeder updates this same user rather
// than creating duplicates.
func seedUser(ctx context.Context, q *db.Queries) db.User {
	u, err := q.UpsertUserByStravaID(ctx, db.UpsertUserByStravaIDParams{
		ID:              seedUserID,
		StravaAthleteID: seedStravaAthleteID,
		Name:            "Seed Athlete",
		AvatarUrl:       "https://dgalywyr863hv.cloudfront.net/pictures/athletes/000/000/1/large.jpg",
		AccessToken:     "seed-access-token",
		RefreshToken:    "seed-refresh-token",
		TokenExpiresAt:  time.Now().Add(24 * time.Hour).Unix(),
		CreatedAt:       time.Now().Unix(),
	})
	if err != nil {
		fatal("upsert seed user failed", err)
	}
	return u
}

// seedUserTwo upserts a second fake athlete, distinct from seedUser's — see
// the seedUserTwoID doc comment for why this exists.
func seedUserTwo(ctx context.Context, q *db.Queries) db.User {
	u, err := q.UpsertUserByStravaID(ctx, db.UpsertUserByStravaIDParams{
		ID:              seedUserTwoID,
		StravaAthleteID: seedStravaAthleteTwoID,
		Name:            "Seed Athlete Two",
		AvatarUrl:       "https://dgalywyr863hv.cloudfront.net/pictures/athletes/000/000/2/large.jpg",
		AccessToken:     "seed-access-token-2",
		RefreshToken:    "seed-refresh-token-2",
		TokenExpiresAt:  time.Now().Add(24 * time.Hour).Unix(),
		CreatedAt:       time.Now().Unix(),
	})
	if err != nil {
		fatal("upsert seed user two failed", err)
	}
	return u
}

func f64(v float64) *float64 { return &v }

// daysAgo returns an RFC3339 timestamp n days before now, at a fixed hour so
// activities land in stable, human-readable order.
func daysAgo(n int, hour int) string {
	t := time.Now().AddDate(0, 0, -n)
	t = time.Date(t.Year(), t.Month(), t.Day(), hour, 0, 0, 0, time.UTC)
	return t.Format(time.RFC3339)
}

// seedActivities returns fixture activities spanning three distinct sport
// GROUPS (run, ride, swim — see internal/sports) — not just distinct sport
// TYPES within one group — so sport-grouped leaderboards have something to
// show in local dev. HR/watts/cadence/suffer_score are deliberately left nil
// on a clear subset (activities 2, 5, and 6 below) and deliberately set on
// the rest, so both the "counted N of M" path and the null-skipping path in
// internal/metrics are exercisable just by running this seeder.
func seedActivities() []strava.Activity {
	return []strava.Activity{
		// --- run group: Run, TrailRun ---
		{
			ID:               900001,
			Name:             "Recovery Run",
			Type:             "Run",
			SportType:        "Run",
			Distance:         6000,
			MovingTime:       1800,
			ElapsedTime:      1850,
			StartDate:        daysAgo(2, 7),
			AverageSpeed:     3.33,
			MaxSpeed:         4.2,
			HasHeartrate:     true,
			AverageHeartrate: f64(142),
			MaxHeartrate:     f64(161),
			AverageCadence:   f64(172),
			SufferScore:      f64(38),
		},
		{
			// Deliberately nil HR/watts/cadence/suffer_score: an older
			// activity logged without a strap.
			ID:           900002,
			Name:         "Tempo Run (no HR strap)",
			Type:         "Run",
			SportType:    "Run",
			Distance:     8000,
			MovingTime:   2200,
			ElapsedTime:  2230,
			StartDate:    daysAgo(5, 6),
			AverageSpeed: 3.64,
			MaxSpeed:     4.8,
		},
		{
			ID:                 900003,
			Name:               "Trail Long Run",
			Type:               "Run",
			SportType:          "TrailRun",
			Distance:           12000,
			TotalElevationGain: 320,
			MovingTime:         2600,
			ElapsedTime:        2700,
			StartDate:          daysAgo(9, 6),
			AverageSpeed:       4.62,
			MaxSpeed:           6.1,
			HasHeartrate:       true,
			AverageHeartrate:   f64(151),
			MaxHeartrate:       f64(174),
			AverageCadence:     f64(168),
			SufferScore:        f64(72),
		},

		// --- ride group: Ride, GravelRide ---
		{
			ID:                   900004,
			Name:                 "Morning Commute Ride",
			Type:                 "Ride",
			SportType:            "Ride",
			Distance:             15000,
			TotalElevationGain:   120,
			MovingTime:           2400,
			ElapsedTime:          2600,
			StartDate:            daysAgo(1, 7),
			AverageSpeed:         6.25,
			MaxSpeed:             12.5,
			HasHeartrate:         true,
			AverageHeartrate:     f64(128),
			MaxHeartrate:         f64(155),
			DeviceWatts:          true,
			AverageWatts:         f64(180),
			WeightedAverageWatts: f64(190),
			MaxWatts:             f64(420),
			Kilojoules:           f64(430),
			AverageCadence:       f64(88),
			SufferScore:          f64(45),
		},
		{
			// Deliberately nil: no power meter or HR strap on this bike.
			ID:                 900005,
			Name:               "Gravel Grind",
			Type:               "Ride",
			SportType:          "GravelRide",
			Distance:           35000,
			TotalElevationGain: 480,
			MovingTime:         6300,
			ElapsedTime:        6900,
			StartDate:          daysAgo(6, 8),
			AverageSpeed:       5.56,
			MaxSpeed:           11.0,
		},

		// --- swim group: Swim ---
		{
			// Deliberately nil: pool swims here never carry HR/cadence.
			ID:           900006,
			Name:         "Pool Swim",
			Type:         "Swim",
			SportType:    "Swim",
			Distance:     2000,
			MovingTime:   2400,
			ElapsedTime:  2500,
			StartDate:    daysAgo(3, 6),
			AverageSpeed: 0.83,
			MaxSpeed:     1.1,
		},
		{
			ID:               900007,
			Name:             "Open Water Swim",
			Type:             "Swim",
			SportType:        "Swim",
			Distance:         3000,
			MovingTime:       3600,
			ElapsedTime:      3650,
			StartDate:        daysAgo(12, 9),
			AverageSpeed:     0.83,
			MaxSpeed:         1.3,
			HasHeartrate:     true,
			AverageHeartrate: f64(135),
			MaxHeartrate:     f64(158),
			SufferScore:      f64(55),
		},
	}
}

// seedActivitiesTwo returns a second user's fixture activities. Deliberately
// smaller than seedActivities' set (it doesn't need to be as elaborate), but
// includes a "run" group activity — the same sport group as seed-user-1's
// "30-Day Run Volume" and "10K Effort" goals — so the two users' entries
// land in the same leaderboard/timeline group and the multi-member label
// collision this seed exists to catch is actually exercisable.
func seedActivitiesTwo() []strava.Activity {
	return []strava.Activity{
		// --- run group: overlaps seed-user-1's run-group goals ---
		{
			ID:               900101,
			Name:             "Easy Miles",
			Type:             "Run",
			SportType:        "Run",
			Distance:         9000,
			MovingTime:       2500,
			ElapsedTime:      2550,
			StartDate:        daysAgo(3, 6),
			AverageSpeed:     3.6,
			MaxSpeed:         4.6,
			HasHeartrate:     true,
			AverageHeartrate: f64(148),
			MaxHeartrate:     f64(166),
			AverageCadence:   f64(170),
			SufferScore:      f64(50),
		},
		{
			ID:                 900102,
			Name:               "Hilly Trail Run",
			Type:               "Run",
			SportType:          "TrailRun",
			Distance:           14000,
			TotalElevationGain: 410,
			MovingTime:         3200,
			ElapsedTime:        3300,
			StartDate:          daysAgo(8, 7),
			AverageSpeed:       4.38,
			MaxSpeed:           5.9,
			HasHeartrate:       true,
			AverageHeartrate:   f64(156),
			MaxHeartrate:       f64(179),
			SufferScore:        f64(80),
		},

		// --- ride group: a second sport group for user two ---
		{
			// Deliberately nil HR/watts: a bike with no sensors attached.
			ID:                 900103,
			Name:               "Weekend Ride",
			Type:               "Ride",
			SportType:          "Ride",
			Distance:           25000,
			TotalElevationGain: 200,
			MovingTime:         4200,
			ElapsedTime:        4500,
			StartDate:          daysAgo(4, 9),
			AverageSpeed:       5.95,
			MaxSpeed:           10.2,
		},
	}
}

// seedCampaign creates one sample campaign with no pooled target
// (metric/target_value both NULL) and joins the seed user to it.
func seedCampaign(ctx context.Context, q *db.Queries, user db.User) db.Campaign {
	now := time.Now()
	c, err := q.CreateCampaign(ctx, db.CreateCampaignParams{
		ID:          uuid.NewString(),
		Name:        "September Fitness Challenge",
		Description: "Casual month-long challenge across the whole team — no pooled target, just personal goals.",
		Metric:      sql.NullString{},  // no pooled target
		TargetValue: sql.NullFloat64{}, // no pooled target
		StartDate:   now.AddDate(0, 0, -14).Unix(),
		EndDate:     now.AddDate(0, 0, 16).Unix(),
		CreatedBy:   user.ID,
		CreatedAt:   now.Unix(),
	})
	if err != nil {
		fatal("create seed campaign failed", err)
	}
	if err := q.JoinCampaign(ctx, db.JoinCampaignParams{
		CampaignID: c.ID,
		UserID:     user.ID,
		JoinedAt:   now.Unix(),
	}); err != nil {
		fatal("join seed campaign failed", err)
	}
	return c
}

// seedGoals creates two clause-based goals: a personal AGGREGATE-only
// "volume" goal, and a campaign performance goal built from SINGLE clauses —
// matching the two clause shapes internal/progress evaluates.
func seedGoals(ctx context.Context, q *db.Queries, user db.User, campaign db.Campaign) {
	now := time.Now()
	periodStart := now.AddDate(0, 0, -14).Unix()
	periodEnd := now.AddDate(0, 0, 16).Unix()

	// Personal AGGREGATE-only volume goal: 20km+ of running (Run + TrailRun,
	// via the "run" sport group) over the period.
	volumeGoal, err := q.CreateGoal(ctx, db.CreateGoalParams{
		ID:          uuid.NewString(),
		UserID:      user.ID,
		CampaignID:  sql.NullString{}, // personal, not tied to the campaign
		Name:        "30-Day Run Volume",
		SportGroup:  sql.NullString{String: "run", Valid: true},
		SportType:   sql.NullString{},
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		CreatedAt:   now.Unix(),
	})
	if err != nil {
		fatal("create volume goal failed", err)
	}
	if _, err := q.CreateGoalClause(ctx, db.CreateGoalClauseParams{
		ID:         uuid.NewString(),
		GoalID:     volumeGoal.ID,
		Position:   0,
		Scope:      "aggregate",
		Metric:     "distance",
		Comparator: "gte",
		Value:      20000,
	}); err != nil {
		fatal("create volume goal clause failed", err)
	}

	// Campaign performance goal built from SINGLE clauses: any one run needs
	// to be both far enough and fast enough.
	perfGoal, err := q.CreateGoal(ctx, db.CreateGoalParams{
		ID:          uuid.NewString(),
		UserID:      user.ID,
		CampaignID:  sql.NullString{String: campaign.ID, Valid: true},
		Name:        "10K Effort",
		SportGroup:  sql.NullString{String: "run", Valid: true},
		SportType:   sql.NullString{},
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		CreatedAt:   now.Unix(),
	})
	if err != nil {
		fatal("create performance goal failed", err)
	}
	clauses := []db.CreateGoalClauseParams{
		{ID: uuid.NewString(), GoalID: perfGoal.ID, Position: 0, Scope: "single", Metric: "distance", Comparator: "gte", Value: 10000},
		{ID: uuid.NewString(), GoalID: perfGoal.ID, Position: 1, Scope: "single", Metric: "moving_time", Comparator: "lte", Value: 2700},
	}
	for _, cl := range clauses {
		if _, err := q.CreateGoalClause(ctx, cl); err != nil {
			fatal("create performance goal clause failed", err)
		}
	}
}

// joinCampaignTwo joins the second seed user to the sample campaign,
// mirroring seedCampaign's own JoinCampaign call for the first user.
func joinCampaignTwo(ctx context.Context, q *db.Queries, userTwo db.User, campaign db.Campaign) {
	if err := q.JoinCampaign(ctx, db.JoinCampaignParams{
		CampaignID: campaign.ID,
		UserID:     userTwo.ID,
		JoinedAt:   time.Now().Unix(),
	}); err != nil {
		fatal("join seed campaign (user two) failed", err)
	}
}

// seedGoalsTwo gives the second seed user one campaign goal in the "run"
// sport group — the same group as seed-user-1's campaign "10K Effort" goal —
// so GetCampaignLeaderboard/GetCampaignTimeline have two members to show
// side by side in an overlapping group, exercising fixes 1-2's multi-member
// clause-labeling and (sport group, metric) grouping.
func seedGoalsTwo(ctx context.Context, q *db.Queries, userTwo db.User, campaign db.Campaign) {
	now := time.Now()
	periodStart := now.AddDate(0, 0, -14).Unix()
	periodEnd := now.AddDate(0, 0, 16).Unix()

	goal, err := q.CreateGoal(ctx, db.CreateGoalParams{
		ID:          uuid.NewString(),
		UserID:      userTwo.ID,
		CampaignID:  sql.NullString{String: campaign.ID, Valid: true},
		Name:        "September Run Distance",
		SportGroup:  sql.NullString{String: "run", Valid: true},
		SportType:   sql.NullString{},
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		CreatedAt:   now.Unix(),
	})
	if err != nil {
		fatal("create seed user two goal failed", err)
	}
	if _, err := q.CreateGoalClause(ctx, db.CreateGoalClauseParams{
		ID:         uuid.NewString(),
		GoalID:     goal.ID,
		Position:   0,
		Scope:      "aggregate",
		Metric:     "distance",
		Comparator: "gte",
		Value:      15000,
	}); err != nil {
		fatal("create seed user two goal clause failed", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
