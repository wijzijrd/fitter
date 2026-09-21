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
