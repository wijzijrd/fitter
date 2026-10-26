// resync re-pulls Strava activity history for every user already in
// fitter's database, using internal/ingest.Backfill — the exact same upsert
// path used by the OAuth-callback backfill and the webhook handler. Useful
// after a schema change, or to catch up on activities missed while the
// webhook subscription was down.
//
// Usage:
//
//	go run ./cmd/resync          # incremental: for each user, pulls activities
//	                              # newer than their latest stored activity
//	                              # (via GetLatestActivityStartDateForUser)
//	go run ./cmd/resync -full    # full re-pull: after=0 for every user
//
// CAVEAT — calories will be NULL for anything pulled this way:
// Strava's list endpoint (GET /athlete/activities, which ListActivities and
// therefore Backfill use) does not include calories; only the per-activity
// detail endpoint (GET /activities/:id, used by the webhook path) does.
// UpsertActivity's COALESCE-based upsert means an existing calories value
// already in the database survives a resync untouched, but any activity
// resync pulls in for the FIRST time will have calories permanently NULL
// until a webhook event (or some other detail-endpoint fetch) supplies it.
//
// RATE LIMITS: Strava enforces 100 requests/15 minutes and 1000
// requests/day per API application — shared across every fitter user, not
// per-user. Each user resynced costs at least one request per page of up to
// 200 activities (see internal/ingest.Backfill's page cap), so running this
// against many users — especially with -full against users with long
// histories — can burn through the 15-minute window or the daily cap
// quickly. This tool does not implement its own rate limiter or backoff;
// that's out of scope for this plan, but a future maintainer running it
// against a real user base needs to plan around this ceiling (batch the
// run across multiple invocations, or add limiting before scaling usage).
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/ingest"
	"github.com/fitter/svc/internal/strava"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	full := flag.Bool("full", false, "fully re-pull every user's history (after=0) instead of incremental")
	flag.Parse()

	// The server applies migrations; this tool expects an up-to-date schema.
	sqlDB, err := sql.Open("pgx", mustEnv("DATABASE_URL"))
	if err != nil {
		fatal("open database failed", err)
	}
	defer sqlDB.Close()

	q := db.New(sqlDB)
	stravaClient := strava.NewClient(strava.Config{
		ClientID:     mustEnv("STRAVA_CLIENT_ID"),
		ClientSecret: mustEnv("STRAVA_CLIENT_SECRET"),
	})

	ctx := context.Background()
	users, err := q.ListAllUsers(ctx)
	if err != nil {
		fatal("list users failed", err)
	}
	slog.Info("resync starting", "users", len(users), "full", *full)

	for _, u := range users {
		after := int64(0)
		if !*full {
			after, err = q.GetLatestActivityStartDateForUser(ctx, u.ID)
			if err != nil {
				slog.Error("get latest activity start date failed", "user_id", u.ID, "err", err)
				continue
			}
		}
		count, err := ingest.Backfill(ctx, q, stravaClient, u, after)
		if err != nil {
			slog.Error("resync backfill failed", "user_id", u.ID, "after", after, "err", err)
			continue
		}
		slog.Info("resync complete for user", "user_id", u.ID, "after", after, "count", count)
	}
	slog.Info("resync finished")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fatal("missing required env var", fmt.Errorf("%s", key))
	}
	return v
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
