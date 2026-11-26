package service_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/service"
)

// leaderboardFor builds one campaign where every member chases the same 10 km run goal, logs the
// given distances, and returns the single group's entries in rank order.
func leaderboardFor(t *testing.T, km map[string]float64) []*fitterv1.LeaderboardEntry {
	t.Helper()
	deps := newDeps(t)
	campaigns, feed := service.NewCampaignService(deps), service.NewFeedService(deps)
	start, end := time.Now().Add(-48*time.Hour).Unix(), time.Now().Add(30*24*time.Hour).Unix()

	goal := func() *fitterv1.CreateGoalRequest {
		return &fitterv1.CreateGoalRequest{
			Name:  "Run 10 km",
			Sport: &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_Group{Group: "run"}},
			Clauses: []*fitterv1.GoalClause{{
				Metric: fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE, Scope: fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
				Comparator: fitterv1.Comparator_COMPARATOR_GTE, Value: 10000,
			}},
		}
	}
	var campaignID, viewer string
	var id int64
	first := true
	for name, d := range km {
		u := seedUser(t, deps.Queries, name)
		if first {
			c, err := campaigns.CreateCampaign(ctxAs(u.ID), connect.NewRequest(baseCreateCampaignRequest(start, end)))
			if err != nil {
				t.Fatal(err)
			}
			campaignID, viewer = c.Msg.GetCampaign().GetId(), u.ID
			g := goal()
			g.CampaignId = &campaignID
			if _, err := service.NewGoalService(deps).CreateGoal(ctxAs(u.ID), connect.NewRequest(g)); err != nil {
				t.Fatal(err)
			}
			first = false
		} else if _, err := campaigns.JoinCampaign(ctxAs(u.ID), connect.NewRequest(&fitterv1.JoinCampaignRequest{CampaignId: campaignID, Goals: []*fitterv1.CreateGoalRequest{goal()}})); err != nil {
			t.Fatal(err)
		}
		id++
		seedActivity(t, deps.Queries, id, u.ID, "Run", time.Now().Add(-time.Hour).Unix(), d*1000)
	}
	resp, err := feed.GetCampaignLeaderboard(ctxAs(viewer), connect.NewRequest(&fitterv1.GetCampaignLeaderboardRequest{CampaignId: campaignID}))
	if err != nil {
		t.Fatalf("GetCampaignLeaderboard: %v", err)
	}
	if len(resp.Msg.GetGroups()) != 1 {
		t.Fatalf("groups = %d, want 1", len(resp.Msg.GetGroups()))
	}
	return resp.Msg.GetGroups()[0].GetEntries()
}

func rankOf(entries []*fitterv1.LeaderboardEntry, name string) *fitterv1.LeaderboardEntry {
	for _, e := range entries {
		if e.GetUserName() == name {
			return e
		}
	}
	return nil
}

// Progress is capped at 100%, so everyone past the target shows 100% and is Satisfied.
func TestLeaderboard_OvershootShowsAsExactlyOneHundredPercent(t *testing.T) {
	e := leaderboardFor(t, map[string]float64{"amy": 10, "zed": 25, "bob": 4})
	for name, want := range map[string]float64{"amy": 1, "zed": 1, "bob": 0.4} {
		got := rankOf(e, name)
		if got == nil || got.GetProgressRatio() != want {
			t.Errorf("%s: ratio %v, want %v", name, got.GetProgressRatio(), want)
		}
	}
	if !rankOf(e, "amy").GetSatisfied() || !rankOf(e, "zed").GetSatisfied() || rankOf(e, "bob").GetSatisfied() {
		t.Errorf("satisfied flags wrong: %v", e)
	}
	if e[2].GetUserName() != "bob" || e[2].GetRank() != 3 {
		t.Errorf("the member short of the goal must rank below both finishers, got %v", e)
	}
}

// Finishing members tie on ratio, and the tie is then broken by NAME, not by how far past the
// goal they went: 25 km ranks below 10 km when the names sort that way. This pins today's
// behaviour so a change (rank by overshoot, or share the rank) is a deliberate one.
func TestLeaderboard_FinishersTieBreakAlphabeticallyNotByDistance(t *testing.T) {
	e := leaderboardFor(t, map[string]float64{"amy": 10, "zed": 25})
	if e[0].GetUserName() != "amy" || e[1].GetUserName() != "zed" {
		t.Fatalf("order = %s, %s; this test documents alphabetical tie-breaking", e[0].GetUserName(), e[1].GetUserName())
	}
	if e[0].GetRank() != 1 || e[1].GetRank() != 2 {
		t.Fatalf("ranks = %d, %d; tied finishers currently get distinct ranks", e[0].GetRank(), e[1].GetRank())
	}
	// The distance each actually ran is still reported, so a UI could show the overshoot.
	if got := rankOf(e, "zed").GetClauses()[0].GetCurrentValue(); got != 25000 {
		t.Fatalf("zed's current value = %v, want the true 25000 m", got)
	}
}
