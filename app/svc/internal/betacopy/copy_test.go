package betacopy_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/fitter/svc/internal/betacopy"
	"github.com/fitter/svc/internal/dbtest"
)

// Two migrated schemas stand in for staging (src) and production (dst). Seeded exercises get
// a different random id in each, which is what the copy has to cope with.
func setup(t *testing.T) (src, dst *sql.DB) {
	t.Helper()
	src, _ = dbtest.New(t)
	dst, _ = dbtest.New(t)
	return src, dst
}

func exec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// seedSource: alice (beta, admin, with a manual session and a custom exercise) and bob (not copied).
func seedSource(t *testing.T, src *sql.DB) {
	t.Helper()
	exec(t, src, `INSERT INTO users (id, strava_athlete_id, name, access_token, refresh_token, token_expires_at, created_at, role, status, feed_show_pace, whats_new_seen, email)
		VALUES ('alice', 1, 'Alice', 'live-a', 'refresh-a', 9999999999, 1, 'admin', 'active', false, '1.0.0', 'alice@example.com'),
		       ('bob',   2, 'Bob',   'live-b', 'refresh-b', 9999999999, 1, 'member', 'active', true, '', '')`)
	// A Strava activity, which must not be copied.
	exec(t, src, `INSERT INTO activities (strava_activity_id, user_id, type, start_date, created_at, updated_at, source)
		VALUES (100, 'alice', 'Run', 1, 1, 1, 'strava')`)
	// A manual session with one lift and one block holding one station on 'Bench press'.
	exec(t, src, `INSERT INTO activities (strava_activity_id, user_id, type, start_date, created_at, updated_at, source)
		VALUES (-1, 'alice', 'WeightTraining', 1, 1, 1, 'manual')`)
	exec(t, src, `INSERT INTO activity_lifts (activity_id, exercise, weight_kg, reps) VALUES (-1, 'Squat', 100, 5)`)
	exec(t, src, `INSERT INTO activity_blocks (activity_id, position, rounds) VALUES (-1, 0, 3)`)
	exec(t, src, `INSERT INTO activity_stations (block_id, position, exercise_id, reps)
		SELECT b.id, 0, e.id, 10 FROM activity_blocks b, exercises e WHERE b.activity_id = -1 AND e.key = 'bench press'`)
	// A user-made exercise by alice, a custom sport, and a campaign goal that must not be copied.
	exec(t, src, `INSERT INTO exercises (id, name, key, measure, muscles, created_by, created_at)
		VALUES ('ex-tgu', 'Turkish get-up', 'turkish get-up', 'weight_reps', ARRAY['core','shoulders'], 'alice', 1)`)
	exec(t, src, `INSERT INTO custom_sports (id, key, label, created_by, created_at) VALUES ('cs1', 'powerlifting', 'Powerlifting', 'alice', 1)`)
	exec(t, src, `INSERT INTO campaigns (id, name, start_date, end_date, created_by, created_at) VALUES ('c1', 'Spring', 1, 2, 'alice', 1)`)
	exec(t, src, `INSERT INTO campaign_members (campaign_id, user_id, joined_at) VALUES ('c1', 'alice', 1)`)
	exec(t, src, `INSERT INTO goals (id, user_id, period_start, period_end, created_at) VALUES ('g1', 'alice', 1, 2, 1)`)
}

func TestCopy_CopiesBetaDataAndNothingElse(t *testing.T) {
	src, dst := setup(t)
	seedSource(t, src)
	ctx := context.Background()

	rep, err := betacopy.Copy(ctx, src, dst, betacopy.Options{AthleteIDs: []int64{1}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep["users"] != 1 || rep["activities"] != 1 || rep["activity_stations"] != 1 {
		t.Fatalf("report = %v", rep)
	}

	// Identity, role, settings and what's-new carry over; Strava tokens are blank.
	var name, role, whatsNew, email, access, refresh string
	var expires int64
	var pace bool
	if err := dst.QueryRow(`SELECT name, role, whats_new_seen, email, access_token, refresh_token, token_expires_at, feed_show_pace
		FROM users WHERE id = 'alice'`).Scan(&name, &role, &whatsNew, &email, &access, &refresh, &expires, &pace); err != nil {
		t.Fatal(err)
	}
	if name != "Alice" || role != "admin" || whatsNew != "1.0.0" || email != "alice@example.com" || pace {
		t.Fatalf("alice = %q %q %q %q pace=%v", name, role, whatsNew, email, pace)
	}
	if access != "" || refresh != "" || expires != 0 {
		t.Fatalf("tokens not blanked: %q %q %d", access, refresh, expires)
	}
	if n := count(t, dst, "users"); n != 1 {
		t.Fatalf("users on dst = %d, want only alice", n)
	}

	// Manual history comes across whole, Strava history and the rest stay out.
	if n := count(t, dst, "activities"); n != 1 {
		t.Fatalf("activities = %d, want 1 manual", n)
	}
	var lifts, stations int
	dst.QueryRow(`SELECT count(*) FROM activity_lifts`).Scan(&lifts)
	dst.QueryRow(`SELECT count(*) FROM activity_stations`).Scan(&stations)
	if lifts != 1 || stations != 1 {
		t.Fatalf("lifts=%d stations=%d", lifts, stations)
	}
	for _, table := range []string{"campaigns", "campaign_members", "goals"} {
		if n := count(t, dst, table); n != 0 {
			t.Fatalf("%s copied: %d rows", table, n)
		}
	}

	// The station points at the destination's own 'Bench press' id, not the source's.
	var stationEx, dstBench string
	dst.QueryRow(`SELECT s.exercise_id FROM activity_stations s`).Scan(&stationEx)
	dst.QueryRow(`SELECT id FROM exercises WHERE key = 'bench press'`).Scan(&dstBench)
	if stationEx != dstBench {
		t.Fatalf("station exercise %q, want dst bench %q", stationEx, dstBench)
	}

	// User-made exercise, custom sport and its metadata come across.
	var created, muscles string
	dst.QueryRow(`SELECT created_by, array_to_string(muscles, ',') FROM exercises WHERE id = 'ex-tgu'`).Scan(&created, &muscles)
	if created != "alice" || muscles != "core,shoulders" {
		t.Fatalf("exercise = %q %q", created, muscles)
	}
	if n := count(t, dst, "custom_sports"); n != 1 {
		t.Fatalf("custom_sports = %d", n)
	}
}

func TestCopy_RerunChangesNothing(t *testing.T) {
	src, dst := setup(t)
	seedSource(t, src)
	ctx := context.Background()
	opt := betacopy.Options{AthleteIDs: []int64{1}}
	if _, err := betacopy.Copy(ctx, src, dst, opt, false); err != nil {
		t.Fatal(err)
	}
	// Alice signs in on production in between; her live tokens must survive a second copy.
	exec(t, dst, `UPDATE users SET access_token = 'prod-live', token_expires_at = 9999999999 WHERE id = 'alice'`)

	if _, err := betacopy.Copy(ctx, src, dst, opt, false); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int{"users": 1, "activities": 1, "activity_lifts": 1, "activity_blocks": 1, "activity_stations": 1, "custom_sports": 1} {
		if got := count(t, dst, table); got != want {
			t.Fatalf("after rerun %s = %d, want %d", table, got, want)
		}
	}
	var access string
	dst.QueryRow(`SELECT access_token FROM users WHERE id = 'alice'`).Scan(&access)
	if access != "prod-live" {
		t.Fatalf("rerun overwrote live token: %q", access)
	}
}

func TestCopy_NewManualActivitiesCannotCollide(t *testing.T) {
	src, dst := setup(t)
	seedSource(t, src)
	ctx := context.Background()
	if _, err := betacopy.Copy(ctx, src, dst, betacopy.Options{AthleteIDs: []int64{1}}, false); err != nil {
		t.Fatal(err)
	}
	var next int64
	if err := dst.QueryRow(`SELECT nextval('manual_activity_id')`).Scan(&next); err != nil {
		t.Fatal(err)
	}
	if next >= -1 {
		t.Fatalf("next manual id %d would collide with copied -1 or be above it", next)
	}
}

func TestCopy_DryRunWritesNothing(t *testing.T) {
	src, dst := setup(t)
	seedSource(t, src)
	rep, err := betacopy.Copy(context.Background(), src, dst, betacopy.Options{All: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep["users"] != 2 || rep["activities"] != 1 {
		t.Fatalf("dry-run counts = %v", rep)
	}
	for _, table := range []string{"users", "activities", "activity_lifts", "custom_sports"} {
		if n := count(t, dst, table); n != 0 {
			t.Fatalf("dry run wrote %s: %d rows", table, n)
		}
	}
}

func TestCopy_RefusesUnknownAthleteAndEmptySelection(t *testing.T) {
	src, dst := setup(t)
	seedSource(t, src)
	ctx := context.Background()
	if _, err := betacopy.Copy(ctx, src, dst, betacopy.Options{}, false); err == nil {
		t.Fatal("empty selection accepted")
	}
	if _, err := betacopy.Copy(ctx, src, dst, betacopy.Options{AthleteIDs: []int64{1, 999}}, false); err == nil {
		t.Fatal("unknown athlete accepted")
	}
	if n := count(t, dst, "users"); n != 0 {
		t.Fatalf("a refused copy still wrote %d users", n)
	}
}

func TestCleanup_DeletesOnlyCopiedUsers(t *testing.T) {
	src, dst := setup(t)
	seedSource(t, src)
	ctx := context.Background()
	if _, err := betacopy.Copy(ctx, src, dst, betacopy.Options{AthleteIDs: []int64{1}}, false); err != nil {
		t.Fatal(err)
	}

	res, err := betacopy.Cleanup(ctx, src, dst, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Names) != 1 || res.Names[0] != "Alice" || res.Report["activities"] != 2 {
		t.Fatalf("dry-run cleanup = %v %v", res.Names, res.Report)
	}
	if n := count(t, src, "users"); n != 2 {
		t.Fatalf("dry run deleted users: %d left", n)
	}

	if _, err := betacopy.Cleanup(ctx, src, dst, false); err != nil {
		t.Fatal(err)
	}
	var left string
	if err := src.QueryRow(`SELECT id FROM users`).Scan(&left); err != nil || left != "bob" {
		t.Fatalf("source users left = %q, %v; want only bob", left, err)
	}
	if n := count(t, src, "activities"); n != 0 {
		t.Fatalf("alice's activities survived cleanup: %d", n)
	}
}
