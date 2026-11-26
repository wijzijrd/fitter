package handler

import (
	"context"
	"testing"
	"time"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/dbtest"
)

// admitAthlete decides whether a Strava sign-in may create or reach an account in each mode.
func TestAdmitAthlete_ByRegistrationMode(t *testing.T) {
	_, q := dbtest.New(t)
	ctx := context.Background()
	d := &Deps{Queries: q, AdminStravaAthleteIDs: []int64{99}}

	// A member already on the platform, created while open. Used to check that invite-only
	// never locks out an existing account.
	if _, err := q.UpsertUserByStravaID(ctx, db.UpsertUserByStravaIDParams{
		ID: "existing", StravaAthleteID: 1, Name: "old", AccessToken: "a", RefreshToken: "r",
		TokenExpiresAt: time.Now().Add(time.Hour).Unix(), CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	// Admin used as the allowlist's added_by FK.
	if _, err := q.UpsertUserByStravaIDAsAdmin(ctx, db.UpsertUserByStravaIDAsAdminParams{
		ID: "admin", StravaAthleteID: 99, Name: "admin", AccessToken: "a", RefreshToken: "r",
		TokenExpiresAt: time.Now().Add(time.Hour).Unix(), CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	check := func(athlete int64, wantAdmitted, wantVouched bool) {
		t.Helper()
		admitted, vouched, err := d.admitAthlete(ctx, athlete)
		if err != nil || admitted != wantAdmitted || vouched != wantVouched {
			t.Fatalf("athlete %d: got admitted=%v vouched=%v err=%v, want %v %v", athlete, admitted, vouched, err, wantAdmitted, wantVouched)
		}
	}

	check(2, true, false) // open: anyone, but not vouched, so pending
	if err := q.SetRegistrationMode(ctx, "invite_only"); err != nil {
		t.Fatal(err)
	}
	check(2, false, false) // unknown, not listed: refused
	check(1, true, false)  // existing account signs in normally
	check(99, true, true)  // bootstrap admin always gets in
	if err := q.AddToRegistrationAllowlist(ctx, db.AddToRegistrationAllowlistParams{StravaAthleteID: 2, AddedBy: "admin", AddedAt: 1}); err != nil {
		t.Fatal(err)
	}
	check(2, true, true) // listed: admitted and vouched
}
