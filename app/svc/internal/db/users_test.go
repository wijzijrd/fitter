package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/dbtest"
)

func upsertParams(id string, athleteID int64) db.UpsertUserByStravaIDParams {
	return db.UpsertUserByStravaIDParams{
		ID:              id,
		StravaAthleteID: athleteID,
		Name:            "Athlete",
		AvatarUrl:       "https://example.com/avatar.png",
		AccessToken:     "access-token",
		RefreshToken:    "refresh-token",
		TokenExpiresAt:  time.Now().Add(time.Hour).Unix(),
		CreatedAt:       time.Now().Unix(),
	}
}

func upsertAdminParams(id string, athleteID int64) db.UpsertUserByStravaIDAsAdminParams {
	p := upsertParams(id, athleteID)
	return db.UpsertUserByStravaIDAsAdminParams{
		ID:              p.ID,
		StravaAthleteID: p.StravaAthleteID,
		Name:            p.Name,
		AvatarUrl:       p.AvatarUrl,
		AccessToken:     p.AccessToken,
		RefreshToken:    p.RefreshToken,
		TokenExpiresAt:  p.TokenExpiresAt,
		CreatedAt:       p.CreatedAt,
	}
}

// TestUpsertUserByStravaID_DefaultRole confirms a brand new user gets the
// column's own 'member' default when the plain (non-admin) upsert path is
// used.
func TestUpsertUserByStravaID_DefaultRole(t *testing.T) {
	_, q := dbtest.New(t)
	u, err := q.UpsertUserByStravaID(context.Background(), upsertParams("user-1", 100))
	if err != nil {
		t.Fatalf("UpsertUserByStravaID: %v", err)
	}
	if u.Role != "member" {
		t.Fatalf("Role = %q, want %q", u.Role, "member")
	}
}

// TestUpsertUserByStravaIDAsAdmin_GrantsAdmin confirms the admin upsert path
// grants role='admin' on first insert.
func TestUpsertUserByStravaIDAsAdmin_GrantsAdmin(t *testing.T) {
	_, q := dbtest.New(t)
	u, err := q.UpsertUserByStravaIDAsAdmin(context.Background(), upsertAdminParams("user-1", 100))
	if err != nil {
		t.Fatalf("UpsertUserByStravaIDAsAdmin: %v", err)
	}
	if u.Role != "admin" {
		t.Fatalf("Role = %q, want %q", u.Role, "admin")
	}
}

// TestAdminRolePersistsAfterRemovalFromEnvVar is the critical correctness
// scenario from Task 3's brief: an athlete id in ADMIN_STRAVA_ATHLETE_IDS
// logs in for the first time (admin upsert path, grants role='admin'). The
// id is then removed from the env var and the same athlete logs in again
// (plain, non-admin upsert path) — they must STAY admin, not get silently
// demoted back to 'member', because the plain path never mentions the role
// column at all.
func TestAdminRolePersistsAfterRemovalFromEnvVar(t *testing.T) {
	_, q := dbtest.New(t)
	ctx := context.Background()

	// First login: athlete id is in ADMIN_STRAVA_ATHLETE_IDS.
	u, err := q.UpsertUserByStravaIDAsAdmin(ctx, upsertAdminParams("user-1", 100))
	if err != nil {
		t.Fatalf("UpsertUserByStravaIDAsAdmin: %v", err)
	}
	if u.Role != "admin" {
		t.Fatalf("after first (admin) login, Role = %q, want %q", u.Role, "admin")
	}

	// Second login: athlete id has since been removed from
	// ADMIN_STRAVA_ATHLETE_IDS, so the caller now takes the plain upsert
	// path — same strava_athlete_id, so this hits the ON CONFLICT branch.
	u, err = q.UpsertUserByStravaID(ctx, upsertParams("user-1", 100))
	if err != nil {
		t.Fatalf("UpsertUserByStravaID (re-login): %v", err)
	}
	if u.Role != "admin" {
		t.Fatalf("after removal from env var and re-login, Role = %q, want %q (must not be silently demoted)", u.Role, "admin")
	}
}
