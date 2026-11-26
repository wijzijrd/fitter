package service_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/service"
	"github.com/fitter/svc/internal/strava"
)

func TestSyncActivities_ImportsNewOnesAndThrottles(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	bob := seedUser(t, deps.Queries, "bob")

	start := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	remote := []strava.Activity{
		{ID: 9001, Name: "Run", Type: "Run", SportType: "Run", Distance: 5000, MovingTime: 1500, ElapsedTime: 1500, StartDate: start, StartDateLocal: start},
		{ID: 9002, Name: "Ride", Type: "Ride", SportType: "Ride", Distance: 20000, MovingTime: 3000, ElapsedTime: 3000, StartDate: start, StartDateLocal: start},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page != "1" {
			_ = json.NewEncoder(w).Encode([]strava.Activity{})
			return
		}
		_ = json.NewEncoder(w).Encode(remote)
	}))
	defer srv.Close()
	deps.Strava = strava.NewClient(strava.Config{APIBaseURL: srv.URL})
	svc := service.NewActivityService(deps)

	sync := func(id string) *fitterv1.SyncActivitiesResponse {
		t.Helper()
		r, err := svc.SyncActivities(ctxAs(id), connect.NewRequest(&fitterv1.SyncActivitiesRequest{}))
		if err != nil {
			t.Fatalf("SyncActivities: %v", err)
		}
		return r.Msg
	}
	reopen := func(id string) { // pretend the five minutes passed
		if _, err := deps.DB.Exec(`UPDATE users SET last_synced_at = 0 WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
	}

	if r := sync(alice.ID); r.Throttled || r.Imported != 2 {
		t.Fatalf("first sync = %+v, want 2 imported", r)
	}
	if r := sync(alice.ID); !r.Throttled || r.Imported != 0 {
		t.Fatalf("immediate second sync = %+v, want throttled", r)
	}

	reopen(alice.ID)
	if r := sync(alice.ID); r.Throttled || r.Imported != 0 {
		t.Fatalf("re-sync of known activities = %+v, want 0 imported, not throttled", r)
	}

	// A watch syncs a workout late: it starts before the newest stored activity.
	late := time.Now().Add(-5 * time.Hour).UTC().Format(time.RFC3339)
	remote = append(remote, strava.Activity{ID: 9003, Name: "Swim", Type: "Swim", SportType: "Swim", Distance: 1000, MovingTime: 1800, ElapsedTime: 1800, StartDate: late, StartDateLocal: late})
	reopen(alice.ID)
	if r := sync(alice.ID); r.Imported != 1 {
		t.Fatalf("late activity sync = %+v, want 1 imported", r)
	}

	// The throttle is per member.
	if r := sync(bob.ID); r.Throttled {
		t.Fatalf("bob throttled by alice's sync")
	}
}
