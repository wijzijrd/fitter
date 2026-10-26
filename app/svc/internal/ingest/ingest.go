// Package ingest is the single write path from Strava into fitter's
// database. Every place a strava.Activity becomes a db row — the
// OAuth-callback backfill, the webhook push, cmd/resync, and cmd/seed — goes
// through UpsertActivity here, and every place an access token gets
// refreshed goes through EnsureFreshAccessToken here. Collapsing these into
// one package means a schema or mapping change (or a token-refresh bugfix)
// happens in exactly one place instead of three.
package ingest

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/strava"
)

// refreshBuffer is how far ahead of actual expiry we proactively refresh, so
// a request never races a token that expires mid-call.
const refreshBuffer = 5 * time.Minute

// UpsertActivity is the ONE place a strava.Activity becomes a DB row. Every
// ingestion path (webhook, backfill, resync, seed) must call this — none may
// build db.UpsertActivityParams directly, so the mapping can never drift
// between callers.
func UpsertActivity(ctx context.Context, q *db.Queries, userID string, a strava.Activity) error {
	startDate, err := time.Parse(time.RFC3339, a.StartDate)
	if err != nil {
		// Deliberately an error, not a silent fallback to time.Now(): a
		// mis-parsed date would otherwise drop the activity into today's
		// date, which can put it in the wrong goal/campaign period without
		// any indication anything went wrong. Callers must skip and log.
		return fmt.Errorf("parse start_date %q for strava activity %d: %w", a.StartDate, a.ID, err)
	}

	// Very old activities predate sport_type; fall back to the legacy type.
	sportType := a.SportType
	if sportType == "" {
		sportType = a.Type
	}

	now := time.Now().Unix()
	_, err = q.UpsertActivity(ctx, db.UpsertActivityParams{
		StravaActivityID:     a.ID,
		UserID:               userID,
		Type:                 a.Type,
		SportType:            sportType,
		Name:                 a.Name,
		StartDate:            startDate.Unix(),
		DistanceM:            a.Distance,
		ElevationM:           a.TotalElevationGain,
		MovingTimeS:          a.MovingTime,
		ElapsedTimeS:         a.ElapsedTime,
		AverageSpeedMs:       a.AverageSpeed,
		MaxSpeedMs:           a.MaxSpeed,
		AverageHeartrate:     nullF(a.AverageHeartrate),
		MaxHeartrate:         nullF(a.MaxHeartrate),
		AverageWatts:         nullF(a.AverageWatts),
		WeightedAverageWatts: nullF(a.WeightedAverageWatts),
		MaxWatts:             nullF(a.MaxWatts),
		DeviceWatts:          a.DeviceWatts,
		Kilojoules:           nullF(a.Kilojoules),
		Calories:             nullF(a.Calories),
		AverageCadence:       nullF(a.AverageCadence),
		SufferScore:          nullF(a.SufferScore),
		CreatedAt:            now,
		UpdatedAt:            now,
	})
	return err
}

// nullF converts a *float64 (strava.Activity's representation of "field
// absent") to sql.NullFloat64 (db.UpsertActivityParams' representation of
// the same thing). A nil pointer becomes {Valid: false}.
func nullF(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

// Backfill pages through a user's Strava history from `after` (0 = full
// history, bounded by the page cap below) and upserts everything it finds.
// It refreshes u's access token once up front via EnsureFreshAccessToken —
// a backfill that runs long after the token's ~6-hour expiry would otherwise
// fail mid-page using a stale token. A failure upserting one activity (e.g.
// an unparseable start_date) is logged and skipped rather than aborting the
// whole backfill; a failure listing a page aborts and returns what was
// upserted so far along with the error.
func Backfill(ctx context.Context, q *db.Queries, c *strava.Client, u db.User, after int64) (int, error) {
	accessToken, err := EnsureFreshAccessToken(ctx, q, c, u)
	if err != nil {
		return 0, fmt.Errorf("ensure fresh access token: %w", err)
	}

	const perPage = 50
	const maxPages = 4 // ~200 most recent activities
	count := 0
	for page := 1; page <= maxPages; page++ {
		activities, err := c.ListActivities(ctx, accessToken, strava.ListOptions{
			Page:    page,
			PerPage: perPage,
			After:   after,
		})
		if err != nil {
			return count, fmt.Errorf("list activities page %d: %w", page, err)
		}
		if len(activities) == 0 {
			return count, nil
		}
		for _, a := range activities {
			if err := UpsertActivity(ctx, q, u.ID, a); err != nil {
				slog.Error("backfill upsert activity failed", "user_id", u.ID, "activity_id", a.ID, "err", err)
				continue
			}
			count++
		}
		if len(activities) < perPage {
			return count, nil
		}
	}
	return count, nil
}

// EnsureFreshAccessToken returns a Strava access token for u guaranteed
// valid for at least refreshBuffer, refreshing and persisting it first if
// needed. This is the single implementation — it used to be duplicated in
// handler/webhook.go and service/tokens.go; both call this now.
func EnsureFreshAccessToken(ctx context.Context, q *db.Queries, c *strava.Client, u db.User) (string, error) {
	if time.Now().Add(refreshBuffer).Before(time.Unix(u.TokenExpiresAt, 0)) {
		return u.AccessToken, nil
	}
	tok, err := c.RefreshAccessToken(ctx, u.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("refresh strava token: %w", err)
	}
	if err := q.UpdateUserTokens(ctx, db.UpdateUserTokensParams{
		AccessToken:    tok.AccessToken,
		RefreshToken:   tok.RefreshToken,
		TokenExpiresAt: tok.ExpiresAt,
		ID:             u.ID,
	}); err != nil {
		return "", fmt.Errorf("persist refreshed token: %w", err)
	}
	return tok.AccessToken, nil
}
