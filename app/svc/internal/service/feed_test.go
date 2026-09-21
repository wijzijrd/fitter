package service_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/service"
)

func TestGetCampaignLeaderboard_GroupsBySportIndependently(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	bob := seedUser(t, deps.Queries, "bob")
	campaignSvc := service.NewCampaignService(deps)
	goalSvc := service.NewGoalService(deps)
	feedSvc := service.NewFeedService(deps)

	start := time.Now().Add(-24 * time.Hour).Unix()
	end := time.Now().Add(30 * 24 * time.Hour).Unix()
	createResp, err := campaignSvc.CreateCampaign(ctxAs(alice.ID), connect.NewRequest(baseCreateCampaignRequest(start, end)))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	campaignID := createResp.Msg.GetCampaign().GetId()

	// alice: a "ride" goal. bob: a "run" goal in the same campaign.
	campaignIDPtr := campaignID
	if _, err := goalSvc.CreateGoal(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateGoalRequest{
		CampaignId: &campaignIDPtr,
		Name:       "alice ride goal",
		Sport:      &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_Group{Group: "ride"}},
		Clauses: []*fitterv1.GoalClause{{
			Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE,
			Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
			Comparator: fitterv1.Comparator_COMPARATOR_GTE,
			Value:      10000,
		}},
	})); err != nil {
		t.Fatalf("CreateGoal(alice): %v", err)
	}

	if _, err := campaignSvc.JoinCampaign(ctxAs(bob.ID), connect.NewRequest(&fitterv1.JoinCampaignRequest{
		CampaignId: campaignID,
		Goals: []*fitterv1.CreateGoalRequest{{
			Name:  "bob run goal",
			Sport: &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_Group{Group: "run"}},
			Clauses: []*fitterv1.GoalClause{{
				Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE,
				Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
				Comparator: fitterv1.Comparator_COMPARATOR_GTE,
				Value:      5000,
			}},
		}},
	})); err != nil {
		t.Fatalf("JoinCampaign(bob): %v", err)
	}

	resp, err := feedSvc.GetCampaignLeaderboard(ctxAs(alice.ID), connect.NewRequest(&fitterv1.GetCampaignLeaderboardRequest{CampaignId: campaignID}))
	if err != nil {
		t.Fatalf("GetCampaignLeaderboard: %v", err)
	}
	groups := resp.Msg.GetGroups()
	if len(groups) != 2 {
		t.Fatalf("GetCampaignLeaderboard: got %d groups, want 2", len(groups))
	}
	for _, g := range groups {
		if len(g.GetEntries()) != 1 {
			t.Errorf("group %q has %d entries, want 1", g.GetSportGroup(), len(g.GetEntries()))
		}
		if g.GetEntries()[0].GetRank() != 1 {
			t.Errorf("group %q entry rank = %d, want 1 (ranked within its own group)", g.GetSportGroup(), g.GetEntries()[0].GetRank())
		}
	}

	// No campaign target was set: every pooled field must be unset.
	if resp.Msg.PooledProgressValue != nil {
		t.Errorf("PooledProgressValue = %v, want nil (no campaign target)", *resp.Msg.PooledProgressValue)
	}
	if resp.Msg.PooledTargetValue != nil {
		t.Errorf("PooledTargetValue = %v, want nil (no campaign target)", *resp.Msg.PooledTargetValue)
	}
	if resp.Msg.PooledMetric != nil {
		t.Errorf("PooledMetric = %v, want nil (no campaign target)", *resp.Msg.PooledMetric)
	}
}

// TestGetCampaignLeaderboard_PooledMetricUnsetWhenNoContributingActivities
// guards against feed.go discarding the pooled accumulator's `ok` flag: a
// nullable pooled metric (average_heartrate) with zero contributing
// activities must leave pooled_progress_value/pooled_target_value/
// pooled_metric all UNSET, matching the timeline's already-correct sibling
// handling of the same case — never render a misleading 0.
func TestGetCampaignLeaderboard_PooledMetricUnsetWhenNoContributingActivities(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	campaignSvc := service.NewCampaignService(deps)
	feedSvc := service.NewFeedService(deps)

	start := time.Now().Add(-24 * time.Hour).Unix()
	end := time.Now().Add(30 * 24 * time.Hour).Unix()
	req := baseCreateCampaignRequest(start, end)
	metric := fitterv1.ActivityMetric_ACTIVITY_METRIC_AVERAGE_HEARTRATE
	target := 140.0
	req.Metric = &metric
	req.TargetValue = &target

	createResp, err := campaignSvc.CreateCampaign(ctxAs(alice.ID), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	campaignID := createResp.Msg.GetCampaign().GetId()

	// No activities at all in the campaign window: the weighted-average
	// accumulator behind average_heartrate has zero contributing rows, so
	// Result()'s ok must come back false.
	resp, err := feedSvc.GetCampaignLeaderboard(ctxAs(alice.ID), connect.NewRequest(&fitterv1.GetCampaignLeaderboardRequest{CampaignId: campaignID}))
	if err != nil {
		t.Fatalf("GetCampaignLeaderboard: %v", err)
	}
	if resp.Msg.PooledProgressValue != nil {
		t.Errorf("PooledProgressValue = %v, want nil (no contributing activities)", *resp.Msg.PooledProgressValue)
	}
	if resp.Msg.PooledTargetValue != nil {
		t.Errorf("PooledTargetValue = %v, want nil (no contributing activities)", *resp.Msg.PooledTargetValue)
	}
	if resp.Msg.PooledMetric != nil {
		t.Errorf("PooledMetric = %v, want nil (no contributing activities)", *resp.Msg.PooledMetric)
	}
}

// TestGetCampaignTimeline_GroupsBySportGroupAndMetric guards against the
// whole-branch review's timeline-grouping finding: two members in the same
// sport group but with DIFFERENT AGGREGATE metrics must land in separate
// TimelineGroups, since a shared y-axis across two different metrics would
// be meaningless. GetCampaignTimeline had zero test coverage before this,
// and the seeder only ever exercised one metric per sport group, so this
// collision never surfaced despite nine rounds of task review.
func TestGetCampaignTimeline_GroupsBySportGroupAndMetric(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	bob := seedUser(t, deps.Queries, "bob")
	campaignSvc := service.NewCampaignService(deps)
	goalSvc := service.NewGoalService(deps)
	feedSvc := service.NewFeedService(deps)

	start := time.Now().Add(-24 * time.Hour).Unix()
	end := time.Now().Add(30 * 24 * time.Hour).Unix()
	createResp, err := campaignSvc.CreateCampaign(ctxAs(alice.ID), connect.NewRequest(baseCreateCampaignRequest(start, end)))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	campaignID := createResp.Msg.GetCampaign().GetId()

	// alice: a "run" goal with an AGGREGATE distance clause.
	campaignIDPtr := campaignID
	if _, err := goalSvc.CreateGoal(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateGoalRequest{
		CampaignId: &campaignIDPtr,
		Name:       "alice run distance goal",
		Sport:      &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_Group{Group: "run"}},
		Clauses: []*fitterv1.GoalClause{{
			Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE,
			Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
			Comparator: fitterv1.Comparator_COMPARATOR_GTE,
			Value:      10000,
		}},
	})); err != nil {
		t.Fatalf("CreateGoal(alice): %v", err)
	}

	// bob: also a "run" goal, same sport group, but an AGGREGATE
	// moving_time clause instead — a different metric that can't share
	// alice's y-axis.
	if _, err := campaignSvc.JoinCampaign(ctxAs(bob.ID), connect.NewRequest(&fitterv1.JoinCampaignRequest{
		CampaignId: campaignID,
		Goals: []*fitterv1.CreateGoalRequest{{
			Name:  "bob run time goal",
			Sport: &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_Group{Group: "run"}},
			Clauses: []*fitterv1.GoalClause{{
				Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_MOVING_TIME,
				Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
				Comparator: fitterv1.Comparator_COMPARATOR_GTE,
				Value:      1800,
			}},
		}},
	})); err != nil {
		t.Fatalf("JoinCampaign(bob): %v", err)
	}

	// Give each user one qualifying "Run" activity inside the campaign
	// window so each goal's AGGREGATE clause produces a timeline point.
	seedActivity(t, deps.Queries, 800001, alice.ID, "Run", time.Now().Unix(), 12000)
	seedActivity(t, deps.Queries, 800002, bob.ID, "Run", time.Now().Unix(), 8000)

	resp, err := feedSvc.GetCampaignTimeline(ctxAs(alice.ID), connect.NewRequest(&fitterv1.GetCampaignTimelineRequest{CampaignId: campaignID}))
	if err != nil {
		t.Fatalf("GetCampaignTimeline: %v", err)
	}

	groups := resp.Msg.GetGroups()
	if len(groups) != 2 {
		t.Fatalf("GetCampaignTimeline: got %d groups, want 2 (same sport group, different metrics must not share a TimelineGroup)", len(groups))
	}

	var allSeries []*fitterv1.TimelineSeries
	for _, g := range groups {
		if g.GetSportGroup() != "run" {
			t.Errorf("group sport_group = %q, want %q", g.GetSportGroup(), "run")
		}
		if len(g.GetSeries()) != 1 {
			t.Errorf("group has %d series, want exactly 1 (one metric per group)", len(g.GetSeries()))
		}
		allSeries = append(allSeries, g.GetSeries()...)
	}

	if len(allSeries) != 2 {
		t.Fatalf("got %d total series, want 2", len(allSeries))
	}
	if allSeries[0].GetLabel() == allSeries[1].GetLabel() {
		t.Errorf("both series share label %q, want distinguishable labels", allSeries[0].GetLabel())
	}
	if allSeries[0].GetMetric() == allSeries[1].GetMetric() {
		t.Errorf("both series share metric %v, want different metrics (that's the point of the test)", allSeries[0].GetMetric())
	}
}
