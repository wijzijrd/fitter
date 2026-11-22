package service_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/service"
)

func clause(m fitterv1.ActivityMetric, scope fitterv1.ClauseScope, v float64) *fitterv1.GoalClause {
	return &fitterv1.GoalClause{Metric: m, Scope: scope, Comparator: fitterv1.Comparator_COMPARATOR_GTE, Value: v}
}

const (
	agg    = fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE
	single = fitterv1.ClauseScope_CLAUSE_SCOPE_SINGLE
)

func totalGoal(sport string, km float64) *fitterv1.CreateGoalRequest {
	g := &fitterv1.CreateGoalRequest{Name: "Total", Clauses: []*fitterv1.GoalClause{clause(fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE, agg, km*1000)}}
	if sport != "" {
		g.Sport = &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_Group{Group: sport}}
	}
	return g
}

func habitGoal(sport string, daysPerWeek float64) *fitterv1.CreateGoalRequest {
	g := &fitterv1.CreateGoalRequest{
		Name: "Habit", Recurrence: fitterv1.Recurrence_RECURRENCE_WEEKLY,
		Clauses: []*fitterv1.GoalClause{clause(fitterv1.ActivityMetric_ACTIVITY_METRIC_ACTIVE_DAYS, agg, daysPerWeek)},
	}
	if sport != "" {
		g.Sport = &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_Group{Group: sport}}
	}
	return g
}

func effortGoal(sport string, km float64) *fitterv1.CreateGoalRequest {
	g := &fitterv1.CreateGoalRequest{Name: "Effort", Clauses: []*fitterv1.GoalClause{clause(fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE, single, km*1000)}}
	if sport != "" {
		g.Sport = &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_Group{Group: sport}}
	}
	return g
}

// campaignWith makes a campaign that started `daysAgo` days ago and has each named member join
// with one goal. It returns the service deps, the viewer (the first member) and the campaign id.
func campaignWith(t *testing.T, daysAgo int, members []string, goals map[string]*fitterv1.CreateGoalRequest) (*service.Deps, string, string) {
	t.Helper()
	deps := newDeps(t)
	campaigns := service.NewCampaignService(deps)
	start, end := time.Now().Add(-time.Duration(daysAgo)*24*time.Hour).Unix(), time.Now().Add(30*24*time.Hour).Unix()
	var cid, viewer string
	for i, name := range members {
		u := seedUser(t, deps.Queries, name)
		if i == 0 {
			c, err := campaigns.CreateCampaign(ctxAs(u.ID), connect.NewRequest(baseCreateCampaignRequest(start, end)))
			if err != nil {
				t.Fatal(err)
			}
			cid, viewer = c.Msg.GetCampaign().GetId(), u.ID
		}
		if _, err := campaigns.JoinCampaign(ctxAs(u.ID), connect.NewRequest(&fitterv1.JoinCampaignRequest{CampaignId: cid, Goals: []*fitterv1.CreateGoalRequest{goals[name]}})); err != nil && i != 0 {
			t.Fatalf("%s joins: %v", name, err)
		}
	}
	return deps, viewer, cid
}

func boardOf(t *testing.T, deps *service.Deps, viewer, cid string) []*fitterv1.LeaderboardGroup {
	t.Helper()
	r, err := service.NewFeedService(deps).GetCampaignLeaderboard(ctxAs(viewer), connect.NewRequest(&fitterv1.GetCampaignLeaderboardRequest{CampaignId: cid}))
	if err != nil {
		t.Fatal(err)
	}
	return r.Msg.GetGroups()
}

func seedUserID(name string) string { return name + "-id" }

// The beta report: a weekly habit on the leaderboard was scored as a running total ("3 active
// days across the whole campaign"), so someone with 2 active days showed 66%, wrongly. A habit
// is buckets hit over buckets so far.
func TestLeaderboard_HabitIsScoredAsAHabitNotARunningTotal(t *testing.T) {
	deps, viewer, cid := campaignWith(t, 21, []string{"amy"}, map[string]*fitterv1.CreateGoalRequest{"amy": habitGoal("", 1)})
	// One active day, three weeks ago. As a running total "1 active day, target 1" is complete;
	// as a weekly habit most weeks since were missed.
	seedActivity(t, deps.Queries, 1, viewer, "Run", time.Now().Add(-20*24*time.Hour).Unix(), 5000)
	groups := boardOf(t, deps, viewer, cid)
	if len(groups) != 1 || len(groups[0].GetEntries()) != 1 {
		t.Fatalf("groups = %v", groups)
	}
	e := groups[0].GetEntries()[0]
	if e.GetSatisfied() || e.GetProgressRatio() >= 1 {
		t.Fatalf("habit shown as satisfied (ratio %v): it was scored as a running total of active days", e.GetProgressRatio())
	}
	if e.GetProgressRatio() <= 0 {
		t.Fatalf("ratio %v: the week with the activity should still count as hit", e.GetProgressRatio())
	}
}

// A total, a single effort and a habit are different races: each is ranked among its own kind.
func TestLeaderboard_GoalKindsAreRankedApartWithinASportGroup(t *testing.T) {
	deps, viewer, cid := campaignWith(t, 14, []string{"amy", "bob", "cat", "dan"}, map[string]*fitterv1.CreateGoalRequest{
		"amy": totalGoal("run", 100),
		"bob": habitGoal("run", 3),
		"cat": effortGoal("run", 5),
		"dan": totalGoal("run", 50),
	})
	seedActivity(t, deps.Queries, 1, seedUserID("amy"), "Run", time.Now().Add(-48*time.Hour).Unix(), 9000)
	seedActivity(t, deps.Queries, 2, seedUserID("dan"), "Run", time.Now().Add(-48*time.Hour).Unix(), 30000)
	groups := boardOf(t, deps, viewer, cid)

	type g struct{ sport, kind, label string }
	var got []g
	for _, grp := range groups {
		got = append(got, g{grp.GetSportGroup(), grp.GetGoalKind(), grp.GetSportGroupLabel()})
	}
	want := []g{{"run", "total", "Run"}, {"run", "performance", "Run · Single efforts"}, {"run", "habit", "Run · Habits"}}
	if len(got) != len(want) {
		t.Fatalf("groups = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("group %d = %v, want %v", i, got[i], want[i])
		}
	}
	// Two totals share a group and rank by ratio (dan 60% beats amy 9%); each lone goal is rank 1.
	if e := groups[0].GetEntries(); len(e) != 2 || e[0].GetUserName() != "dan" || e[0].GetRank() != 1 || e[1].GetRank() != 2 {
		t.Errorf("totals = %v, want dan then amy", e)
	}
	for _, i := range []int{1, 2} {
		if e := groups[i].GetEntries(); len(e) != 1 || e[0].GetRank() != 1 {
			t.Errorf("group %d = %v, want one entry ranked 1", i, e)
		}
	}
}

// Different sports are still apart, and the sport order stays the primary order.
func TestLeaderboard_SportOrderComesBeforeKindOrder(t *testing.T) {
	deps, viewer, cid := campaignWith(t, 14, []string{"amy", "bob", "cat"}, map[string]*fitterv1.CreateGoalRequest{
		"amy": totalGoal("ride", 100),
		"bob": habitGoal("run", 3),
		"cat": totalGoal("", 100),
	})
	groups := boardOf(t, deps, viewer, cid)
	var order []string
	for _, grp := range groups {
		order = append(order, grp.GetSportGroup()+"/"+grp.GetGoalKind())
	}
	want := []string{"run/habit", "ride/total", "/total"} // run before ride; "all sports" last
	if len(order) != 3 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if lbl := groups[2].GetSportGroupLabel(); lbl != "All sports" {
		t.Errorf("all-sports label = %q", lbl)
	}
}

// The people ranking averages across a member's goals and is unchanged by grouping.
func TestLeaderboard_PeopleRankingStillSpansEveryKind(t *testing.T) {
	deps, viewer, cid := campaignWith(t, 14, []string{"amy", "bob"}, map[string]*fitterv1.CreateGoalRequest{
		"amy": totalGoal("run", 100),
		"bob": habitGoal("run", 3),
	})
	r, err := service.NewFeedService(deps).GetCampaignLeaderboard(ctxAs(viewer), connect.NewRequest(&fitterv1.GetCampaignLeaderboardRequest{CampaignId: cid}))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Msg.GetPeople()) != 2 || len(r.Msg.GetGroups()) != 2 {
		t.Fatalf("people=%d groups=%d, want 2 and 2", len(r.Msg.GetPeople()), len(r.Msg.GetGroups()))
	}
}
