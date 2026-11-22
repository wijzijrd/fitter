package service_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/service"
)

func TestFeedSettings_DefaultsUpdateAndStayPerUser(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	bob := seedUser(t, deps.Queries, "bob")
	svc := service.NewUserService(deps)

	cur, err := svc.GetCurrentUser(ctxAs(alice.ID), connect.NewRequest(&fitterv1.GetCurrentUserRequest{}))
	if f := cur.Msg.GetUser().GetFeedSettings(); err != nil || !f.GetShowPace() || !f.GetShowPower() || !f.GetShowEffort() || !f.GetShowLifts() {
		t.Fatalf("defaults: %v err=%v, want all four on", f, err)
	}
	if _, err := svc.UpdateFeedSettings(ctxAs(alice.ID), connect.NewRequest(&fitterv1.UpdateFeedSettingsRequest{
		FeedSettings: &fitterv1.FeedSettings{ShowPace: false, ShowPower: true, ShowEffort: false, ShowLifts: true},
	})); err != nil {
		t.Fatalf("UpdateFeedSettings: %v", err)
	}
	cur, _ = svc.GetCurrentUser(ctxAs(alice.ID), connect.NewRequest(&fitterv1.GetCurrentUserRequest{}))
	if f := cur.Msg.GetUser().GetFeedSettings(); f.GetShowPace() || !f.GetShowPower() || f.GetShowEffort() || !f.GetShowLifts() {
		t.Fatalf("after update: %v", f)
	}
	other, _ := svc.GetCurrentUser(ctxAs(bob.ID), connect.NewRequest(&fitterv1.GetCurrentUserRequest{}))
	if !other.Msg.GetUser().GetFeedSettings().GetShowPace() {
		t.Fatal("bob's settings changed when alice's did")
	}
}

// The feed carries a manual session's blocks, never heart rate, and remembers when each member last looked.
func TestFeed_BlocksSeenAtAndNoHeartRate(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	bob := seedUser(t, deps.Queries, "bob")
	carol := seedUser(t, deps.Queries, "carol")
	campaigns, feed, acts := service.NewCampaignService(deps), service.NewFeedService(deps), service.NewActivityService(deps)

	start, end := time.Now().Add(-24*time.Hour).Unix(), time.Now().Add(30*24*time.Hour).Unix()
	created, err := campaigns.CreateCampaign(ctxAs(alice.ID), connect.NewRequest(baseCreateCampaignRequest(start, end)))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	cid := created.Msg.GetCampaign().GetId()
	if _, err := campaigns.JoinCampaign(ctxAs(bob.ID), connect.NewRequest(&fitterv1.JoinCampaignRequest{
		CampaignId: cid,
		Goals: []*fitterv1.CreateGoalRequest{{
			Name: "bob goal",
			Clauses: []*fitterv1.GoalClause{{
				Metric: fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE, Scope: fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
				Comparator: fitterv1.Comparator_COMPARATOR_GTE, Value: 5000,
			}},
		}},
	})); err != nil {
		t.Fatalf("JoinCampaign: %v", err)
	}

	if _, err := deps.DB.ExecContext(context.Background(),
		`INSERT INTO activities (strava_activity_id, user_id, type, sport_type, name, start_date, average_heartrate, source, created_at, updated_at)
		 VALUES (555, $1, 'Run', 'Run', 'Easy run', $2, 150, 'strava', 0, 0)`, bob.ID, time.Now().Add(-2*time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := acts.CreateManualActivity(ctxAs(bob.ID), connect.NewRequest(&fitterv1.CreateManualActivityRequest{
		Activity: circuit(&fitterv1.Block{Rounds: 3, Stations: []*fitterv1.Station{stationOf(deps, t, "bench press", func(s *fitterv1.Station) { s.WeightKg, s.Reps = f64(80), i32(5) })}}),
	})); err != nil {
		t.Fatalf("log: %v", err)
	}

	resp, err := feed.GetCampaignFeed(ctxAs(alice.ID), connect.NewRequest(&fitterv1.GetCampaignFeedRequest{CampaignId: cid}))
	if err != nil {
		t.Fatalf("GetCampaignFeed: %v", err)
	}
	var manual, run *fitterv1.FeedActivity
	for _, a := range resp.Msg.GetActivities() {
		if a.GetStravaActivityId() < 0 {
			manual = a
		} else {
			run = a
		}
	}
	if manual == nil || len(manual.GetBlocks()) != 1 || manual.GetBlocks()[0].GetRounds() != 3 || manual.GetBlocks()[0].GetStations()[0].GetExerciseName() != "Bench press" {
		t.Fatalf("manual row = %v, want its block", manual)
	}
	if run == nil || run.AverageHeartrate != nil || len(run.GetBlocks()) != 0 {
		t.Fatalf("strava row = %v, want no heart rate and no blocks", run)
	}

	// First visit has no seen_at; marking moves only the caller's own.
	if resp.Msg.GetSeenAt() != nil {
		t.Fatalf("first view seen_at = %v, want unset", resp.Msg.GetSeenAt())
	}
	if _, err := feed.MarkFeedSeen(ctxAs(alice.ID), connect.NewRequest(&fitterv1.MarkFeedSeenRequest{CampaignId: cid})); err != nil {
		t.Fatalf("MarkFeedSeen: %v", err)
	}
	again, _ := feed.GetCampaignFeed(ctxAs(alice.ID), connect.NewRequest(&fitterv1.GetCampaignFeedRequest{CampaignId: cid}))
	if at := again.Msg.GetSeenAt(); at == nil || time.Since(at.AsTime()) > time.Minute {
		t.Fatalf("seen_at after marking = %v, want now", at)
	}
	bobs, _ := feed.GetCampaignFeed(ctxAs(bob.ID), connect.NewRequest(&fitterv1.GetCampaignFeedRequest{CampaignId: cid}))
	if bobs.Msg.GetSeenAt() != nil {
		t.Fatal("bob's seen_at moved when alice marked hers")
	}
	if _, err := feed.MarkFeedSeen(ctxAs(carol.ID), connect.NewRequest(&fitterv1.MarkFeedSeenRequest{CampaignId: cid})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("non-member mark: got %v, want PermissionDenied", err)
	}
}
