package service_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ts(sec int64) *timestamppb.Timestamp {
	return timestamppb.New(time.Unix(sec, 0).UTC())
}

func seedActivity(t *testing.T, queries *db.Queries, id int64, userID, sportType string, start int64, distanceM float64) {
	t.Helper()
	_, err := queries.UpsertActivity(context.Background(), db.UpsertActivityParams{
		StravaActivityID: id,
		UserID:           userID,
		Type:             sportType,
		SportType:        sportType,
		Name:             "test activity",
		StartDate:        start,
		DistanceM:        distanceM,
		MovingTimeS:      1800,
		ElapsedTimeS:     1800,
		CreatedAt:        start,
		UpdatedAt:        start,
	})
	if err != nil {
		t.Fatalf("seedActivity: %v", err)
	}
}

func twoClauseCreateGoalRequest(periodStart, periodEnd int64) *fitterv1.CreateGoalRequest {
	return &fitterv1.CreateGoalRequest{
		Name:        "Distance + count goal",
		PeriodStart: ts(periodStart),
		PeriodEnd:   ts(periodEnd),
		Clauses: []*fitterv1.GoalClause{
			{
				Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE,
				Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
				Comparator: fitterv1.Comparator_COMPARATOR_GTE,
				Value:      10000,
			},
			{
				Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_ACTIVITY_COUNT,
				Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
				Comparator: fitterv1.Comparator_COMPARATOR_GTE,
				Value:      3,
			},
		},
	}
}

func TestCreateGoal_ListGoals_ClausesAndProgress(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	svc := service.NewGoalService(deps)
	ctx := ctxAs(alice.ID)

	periodStart := time.Now().Add(-7 * 24 * time.Hour).Unix()
	periodEnd := time.Now().Add(7 * 24 * time.Hour).Unix()

	createResp, err := svc.CreateGoal(ctx, connect.NewRequest(twoClauseCreateGoalRequest(periodStart, periodEnd)))
	if err != nil {
		t.Fatalf("CreateGoal: %v", err)
	}
	created := createResp.Msg.GetGoal()
	if created.GetId() == "" {
		t.Fatal("CreateGoal: empty goal id")
	}
	if len(created.GetClauses()) != 2 {
		t.Fatalf("CreateGoal: got %d clauses, want 2", len(created.GetClauses()))
	}

	// One activity: partially satisfies both clauses (5000/10000 distance,
	// 1/3 activity count).
	seedActivity(t, deps.Queries, 1, alice.ID, "Run", time.Now().Unix(), 5000)

	listResp, err := svc.ListGoals(ctx, connect.NewRequest(&fitterv1.ListGoalsRequest{}))
	if err != nil {
		t.Fatalf("ListGoals: %v", err)
	}
	if len(listResp.Msg.GetGoals()) != 1 {
		t.Fatalf("ListGoals: got %d goals, want 1", len(listResp.Msg.GetGoals()))
	}
	g := listResp.Msg.GetGoals()[0]

	clauses := g.GetClauses()
	if len(clauses) != 2 {
		t.Fatalf("ListGoals: got %d clauses, want 2", len(clauses))
	}
	if clauses[0].GetPosition() != 0 || clauses[0].GetMetric() != fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE {
		t.Errorf("clause 0 = %+v, want position 0 distance", clauses[0])
	}
	if clauses[1].GetPosition() != 1 || clauses[1].GetMetric() != fitterv1.ActivityMetric_ACTIVITY_METRIC_ACTIVITY_COUNT {
		t.Errorf("clause 1 = %+v, want position 1 activity_count", clauses[1])
	}

	progress := g.GetProgress()
	if progress == nil {
		t.Fatal("ListGoals: nil progress")
	}
	if progress.GetSatisfied() {
		t.Error("progress.Satisfied = true, want false (distance clause not met)")
	}
	pClauses := progress.GetClauses()
	if len(pClauses) != 2 {
		t.Fatalf("progress clauses = %d, want 2", len(pClauses))
	}
	if pClauses[0].GetCurrentValue() != 5000 {
		t.Errorf("distance clause current value = %v, want 5000", pClauses[0].GetCurrentValue())
	}
	if pClauses[1].GetCurrentValue() != 1 {
		t.Errorf("activity_count clause current value = %v, want 1", pClauses[1].GetCurrentValue())
	}
}

func TestUpdateGoal_ReplacesClauseList(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	svc := service.NewGoalService(deps)
	ctx := ctxAs(alice.ID)

	periodStart := time.Now().Add(-7 * 24 * time.Hour).Unix()
	periodEnd := time.Now().Add(7 * 24 * time.Hour).Unix()

	createResp, err := svc.CreateGoal(ctx, connect.NewRequest(twoClauseCreateGoalRequest(periodStart, periodEnd)))
	if err != nil {
		t.Fatalf("CreateGoal: %v", err)
	}
	goalID := createResp.Msg.GetGoal().GetId()

	oldClauseRows, err := deps.Queries.ListGoalClausesForGoal(context.Background(), goalID)
	if err != nil {
		t.Fatalf("ListGoalClausesForGoal (before): %v", err)
	}
	if len(oldClauseRows) != 2 {
		t.Fatalf("before update: got %d clause rows, want 2", len(oldClauseRows))
	}

	updateReq := &fitterv1.UpdateGoalRequest{
		GoalId:      goalID,
		Name:        "Renamed goal",
		PeriodStart: ts(periodStart),
		PeriodEnd:   ts(periodEnd),
		Clauses: []*fitterv1.GoalClause{
			{
				Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_MOVING_TIME,
				Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
				Comparator: fitterv1.Comparator_COMPARATOR_GTE,
				Value:      100,
			},
		},
	}
	updateResp, err := svc.UpdateGoal(ctx, connect.NewRequest(updateReq))
	if err != nil {
		t.Fatalf("UpdateGoal: %v", err)
	}
	updated := updateResp.Msg.GetGoal()
	if updated.GetName() != "Renamed goal" {
		t.Errorf("updated name = %q, want %q", updated.GetName(), "Renamed goal")
	}
	if len(updated.GetClauses()) != 1 || updated.GetClauses()[0].GetMetric() != fitterv1.ActivityMetric_ACTIVITY_METRIC_MOVING_TIME {
		t.Fatalf("updated clauses = %+v, want single moving_time clause", updated.GetClauses())
	}

	newClauseRows, err := deps.Queries.ListGoalClausesForGoal(context.Background(), goalID)
	if err != nil {
		t.Fatalf("ListGoalClausesForGoal (after): %v", err)
	}
	if len(newClauseRows) != 1 {
		t.Fatalf("after update: got %d clause rows, want 1", len(newClauseRows))
	}
	for _, old := range oldClauseRows {
		for _, cur := range newClauseRows {
			if old.ID == cur.ID {
				t.Errorf("old clause row %s survived UpdateGoal", old.ID)
			}
		}
	}
}

func TestGoal_OwnershipEnforced(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	bob := seedUser(t, deps.Queries, "bob")
	svc := service.NewGoalService(deps)
	aliceCtx := ctxAs(alice.ID)
	bobCtx := ctxAs(bob.ID)

	periodStart := time.Now().Add(-7 * 24 * time.Hour).Unix()
	periodEnd := time.Now().Add(7 * 24 * time.Hour).Unix()

	createResp, err := svc.CreateGoal(aliceCtx, connect.NewRequest(twoClauseCreateGoalRequest(periodStart, periodEnd)))
	if err != nil {
		t.Fatalf("CreateGoal: %v", err)
	}
	goalID := createResp.Msg.GetGoal().GetId()

	// Read: bob's personal goal list must never include alice's goal.
	bobList, err := svc.ListGoals(bobCtx, connect.NewRequest(&fitterv1.ListGoalsRequest{}))
	if err != nil {
		t.Fatalf("ListGoals(bob): %v", err)
	}
	if len(bobList.Msg.GetGoals()) != 0 {
		t.Fatalf("bob's goal list contains %d goals, want 0 (alice's goal must not leak)", len(bobList.Msg.GetGoals()))
	}

	// Update: bob updating alice's goal id must fail as NotFound, never
	// succeed and never leak a distinct "forbidden" signal.
	_, err = svc.UpdateGoal(bobCtx, connect.NewRequest(&fitterv1.UpdateGoalRequest{
		GoalId:      goalID,
		Name:        "hijacked",
		PeriodStart: ts(periodStart),
		PeriodEnd:   ts(periodEnd),
		Clauses: []*fitterv1.GoalClause{{
			Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE,
			Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
			Comparator: fitterv1.Comparator_COMPARATOR_GTE,
			Value:      1,
		}},
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("UpdateGoal(bob, alice's goal) error = %v, want CodeNotFound", err)
	}

	// Delete: bob deleting alice's goal must be a silent no-op (the
	// DELETE's WHERE id=? AND user_id=? just matches zero rows).
	if _, err := svc.DeleteGoal(bobCtx, connect.NewRequest(&fitterv1.DeleteGoalRequest{GoalId: goalID})); err != nil {
		t.Fatalf("DeleteGoal(bob, alice's goal): %v", err)
	}
	aliceList, err := svc.ListGoals(aliceCtx, connect.NewRequest(&fitterv1.ListGoalsRequest{}))
	if err != nil {
		t.Fatalf("ListGoals(alice): %v", err)
	}
	if len(aliceList.Msg.GetGoals()) != 1 {
		t.Fatalf("alice's goal was deleted by bob's request: got %d goals, want 1", len(aliceList.Msg.GetGoals()))
	}
}
