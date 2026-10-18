// seed writes a fake Strava-authenticated user plus a run of sample
// activities directly into the local Postgres DB, then prints a ready-to-use
// JWT — so the app's UI can be exercised end-to-end without ever talking to
// the real Strava API. Run the server once first (it applies migrations),
// then:
//
//	DATABASE_URL=postgres://fitter:fitter@localhost:5432/fitter?sslmode=disable \
//	  JWT_SECRET=<same as server> go run ./cmd/seed
//
// Open FRONTEND_URL + /auth/callback#token=<printed token> to land in the
// app already "logged in" as the fake user.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/google/uuid"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type fakeActivity struct {
	daysAgo    int
	activity   string
	distanceM  float64
	elevationM float64
	movingS    int64
	name       string
}

var fakeActivities = []fakeActivity{
	{1, "Run", 8200, 45, 2400, "Easy morning run"},
	{2, "Ride", 32000, 310, 4800, "Hill loop"},
	{4, "Run", 5000, 12, 1500, "Recovery jog"},
	{6, "Ride", 45000, 520, 6600, "Long Saturday ride"},
	{8, "Run", 10000, 60, 2900, "Tempo run"},
	{10, "Swim", 2000, 0, 2100, "Pool session"},
	{13, "Run", 15000, 180, 4500, "Trail run"},
	{15, "Ride", 20000, 150, 2700, "Commute loop"},
	{18, "Run", 6000, 20, 1800, "Shakeout run"},
	{21, "Ride", 60000, 700, 8400, "Weekend century-ish"},
	{24, "Run", 9000, 55, 2600, "Steady run"},
	{27, "Run", 12000, 90, 3400, "Progression run"},
}

func main() {
	databaseURL := mustEnv("DATABASE_URL")
	jwtSecret := mustEnv("JWT_SECRET")

	sqlDB, err := sql.Open("pgx", databaseURL)
	fatalIf(err)
	defer sqlDB.Close()
	fatalIf(sqlDB.Ping())

	queries := db.New(sqlDB)
	ctx := context.Background()

	u, err := queries.UpsertUserByStravaID(ctx, db.UpsertUserByStravaIDParams{
		ID:              uuid.NewString(),
		StravaAthleteID: 900000001,
		Name:            "Fake Athlete",
		AvatarUrl:       "",
		AccessToken:     "fake-access-token",
		RefreshToken:    "fake-refresh-token",
		TokenExpiresAt:  time.Now().Add(24 * time.Hour).Unix(),
		CreatedAt:       time.Now().Unix(),
	})
	fatalIf(err)

	for i, a := range fakeActivities {
		startDate := time.Now().AddDate(0, 0, -a.daysAgo)
		_, err := queries.UpsertActivity(ctx, db.UpsertActivityParams{
			ID:               uuid.NewString(),
			UserID:           u.ID,
			StravaActivityID: int64(900100000 + i),
			Type:             a.activity,
			DistanceM:        a.distanceM,
			ElevationM:       a.elevationM,
			MovingTimeS:      a.movingS,
			StartDate:        startDate.Unix(),
			Name:             a.name,
			CreatedAt:        time.Now().Unix(),
		})
		fatalIf(err)
	}

	token, err := auth.Issue([]byte(jwtSecret), u.ID, 30*24*time.Hour)
	fatalIf(err)

	fmt.Printf("Seeded fake user %q (id=%s) with %d activities.\n\n", u.Name, u.ID, len(fakeActivities))
	fmt.Println("JWT:")
	fmt.Println(token)
	fmt.Println()
	fmt.Println("Open in browser (replace with your FRONTEND_URL):")
	fmt.Println("  http://localhost:5173/auth/callback#token=" + token)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "missing required env var %s\n", key)
		os.Exit(1)
	}
	return v
}

func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
