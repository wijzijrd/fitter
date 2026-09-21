package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/dbtest"
	"github.com/fitter/svc/internal/service"
)

// newValidNullString is a small literal-builder to keep sql.NullString{...,
// Valid: true} construction out of every test's line-count.
func newValidNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

// newDeps returns a fresh, fully-migrated, in-memory-backed service.Deps for
// one test.
func newDeps(t *testing.T) *service.Deps {
	t.Helper()
	sqlDB, queries := dbtest.New(t)
	return &service.Deps{DB: sqlDB, Queries: queries}
}

var seedSeq int64

// seedUser inserts a user row directly (bypassing the Strava OAuth flow —
// tests only need a row users.id can foreign-key against) and returns it.
func seedUser(t *testing.T, queries *db.Queries, name string) db.User {
	t.Helper()
	seedSeq++
	u, err := queries.UpsertUserByStravaID(context.Background(), db.UpsertUserByStravaIDParams{
		ID:              name + "-id",
		StravaAthleteID: seedSeq,
		Name:            name,
		AvatarUrl:       "https://example.com/" + name + ".png",
		AccessToken:     "access-token",
		RefreshToken:    "refresh-token",
		TokenExpiresAt:  time.Now().Add(time.Hour).Unix(),
		CreatedAt:       time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("seedUser(%q): %v", name, err)
	}
	return u
}

// ctxAs returns a context carrying userID the way the auth interceptor
// would after verifying a bearer token.
func ctxAs(userID string) context.Context {
	return auth.WithUserID(context.Background(), userID)
}
