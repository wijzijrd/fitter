package service_test

import (
	"math"
	"testing"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/service"
)

func manual(lifts ...*fitterv1.Lift) *fitterv1.ManualActivity {
	return &fitterv1.ManualActivity{SportType: "WeightTraining", Name: "Push day", StartDate: ts(time.Now().Add(-time.Hour).Unix()), MovingTimeS: 3600, Lifts: lifts}
}

// A manual bench-press log drives a max-weight goal; another user can't touch the entry.
func TestManualActivity_LiftsDriveMaxWeightGoal(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	bob := seedUser(t, deps.Queries, "bob")
	acts := service.NewActivityService(deps)
	goals := service.NewGoalService(deps)
	ctx := ctxAs(alice.ID)

	created, err := acts.CreateManualActivity(ctx, connect.NewRequest(&fitterv1.CreateManualActivityRequest{
		Activity: manual(&fitterv1.Lift{Exercise: "  Bench   Press", WeightKg: 85, Reps: 5}, &fitterv1.Lift{Exercise: "squat", WeightKg: 120, Reps: 3}),
	}))
	if err != nil {
		t.Fatalf("CreateManualActivity: %v", err)
	}
	a := created.Msg.GetActivity()
	if a.GetId() >= 0 || a.GetLifts()[0].GetExercise() != "bench press" {
		t.Fatalf("id=%d lifts=%v, want negative id and normalised exercise", a.GetId(), a.GetLifts())
	}

	exercise := "Bench Press"
	start, end := time.Now().Add(-24*time.Hour).Unix(), time.Now().Add(24*time.Hour).Unix()
	g, err := goals.CreateGoal(ctx, connect.NewRequest(&fitterv1.CreateGoalRequest{
		Name: "Bench 100", PeriodStart: ts(start), PeriodEnd: ts(end),
		Clauses: []*fitterv1.GoalClause{{
			Metric: fitterv1.ActivityMetric_ACTIVITY_METRIC_MAX_WEIGHT_KG, Scope: fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
			Comparator: fitterv1.Comparator_COMPARATOR_GTE, Value: 100, Exercise: &exercise,
		}},
	}))
	if err != nil {
		t.Fatalf("CreateGoal: %v", err)
	}
	got := g.Msg.GetGoal().GetProgress().GetClauses()[0].GetCurrentValue()
	if got != 85 {
		t.Fatalf("bench max = %v, want 85 (squat must not count)", got)
	}

	// Exercise is required for max weight.
	_, err = goals.CreateGoal(ctx, connect.NewRequest(&fitterv1.CreateGoalRequest{
		Name: "x", PeriodStart: ts(start), PeriodEnd: ts(end),
		Clauses: []*fitterv1.GoalClause{{
			Metric: fitterv1.ActivityMetric_ACTIVITY_METRIC_MAX_WEIGHT_KG, Scope: fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
			Comparator: fitterv1.Comparator_COMPARATOR_GTE, Value: 100,
		}},
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("missing exercise: got %v, want InvalidArgument", err)
	}

	// Ownership: bob can neither delete nor update alice's entry.
	if _, err := acts.DeleteManualActivity(ctxAs(bob.ID), connect.NewRequest(&fitterv1.DeleteManualActivityRequest{Id: a.GetId()})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("bob delete: got %v, want NotFound", err)
	}
	a.Name = "hijack"
	if _, err := acts.UpdateManualActivity(ctxAs(bob.ID), connect.NewRequest(&fitterv1.UpdateManualActivityRequest{Activity: a})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("bob update: got %v, want NotFound", err)
	}

	// Lifts on a non-strength sport are rejected.
	bad := manual(&fitterv1.Lift{Exercise: "squat", WeightKg: 1, Reps: 1})
	bad.SportType = "Run"
	if _, err := acts.CreateManualActivity(ctx, connect.NewRequest(&fitterv1.CreateManualActivityRequest{Activity: bad})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("run with lifts: got %v, want InvalidArgument", err)
	}
}

func TestManualActivity_RejectsBadNumbers(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	acts := service.NewActivityService(deps)
	for name, mod := range map[string]func(*fitterv1.ManualActivity){
		"nan distance":  func(a *fitterv1.ManualActivity) { d := math.NaN(); a.DistanceM = &d },
		"huge distance": func(a *fitterv1.ManualActivity) { d := 1e9; a.DistanceM = &d },
		"future start":  func(a *fitterv1.ManualActivity) { a.StartDate = ts(time.Now().Add(72 * time.Hour).Unix()) },
		"nan weight":    func(a *fitterv1.ManualActivity) { a.Lifts[0].WeightKg = math.NaN() },
		"huge weight":   func(a *fitterv1.ManualActivity) { a.Lifts[0].WeightKg = 1e6 },
		"huge reps":     func(a *fitterv1.ManualActivity) { a.Lifts[0].Reps = 100000 },
	} {
		a := manual(&fitterv1.Lift{Exercise: "squat", WeightKg: 100, Reps: 5})
		mod(a)
		_, err := acts.CreateManualActivity(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateManualActivityRequest{Activity: a}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}
}

// A weekly "4 active days" habit: a manual entry counts toward its local week,
// the goal reports buckets hit/elapsed, and a daily goal over years is refused.
func TestRecurringGoal_ManualEntryCountsAndBucketCapEnforced(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	acts := service.NewActivityService(deps)
	goals := service.NewGoalService(deps)
	ctx := ctxAs(alice.ID)

	off := int32(120)
	for i := 0; i < 2; i++ {
		a := manual()
		a.SportType = "Run"
		a.StartDate = ts(time.Now().Add(-time.Duration(i) * 25 * time.Hour).Unix())
		a.UtcOffsetMinutes = &off
		if _, err := acts.CreateManualActivity(ctx, connect.NewRequest(&fitterv1.CreateManualActivityRequest{Activity: a})); err != nil {
			t.Fatalf("CreateManualActivity: %v", err)
		}
	}
	clause := []*fitterv1.GoalClause{{
		Metric: fitterv1.ActivityMetric_ACTIVITY_METRIC_ACTIVE_DAYS, Scope: fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
		Comparator: fitterv1.Comparator_COMPARATOR_GTE, Value: 1,
	}}
	start, end := time.Now().Add(-48*time.Hour).Unix(), time.Now().Add(7*24*time.Hour).Unix()
	g, err := goals.CreateGoal(ctx, connect.NewRequest(&fitterv1.CreateGoalRequest{
		Name: "Move weekly", PeriodStart: ts(start), PeriodEnd: ts(end), Clauses: clause,
		Recurrence: fitterv1.Recurrence_RECURRENCE_WEEKLY,
	}))
	if err != nil {
		t.Fatalf("CreateGoal: %v", err)
	}
	p := g.Msg.GetGoal().GetProgress()
	if g.Msg.GetGoal().GetRecurrence() != fitterv1.Recurrence_RECURRENCE_WEEKLY || p.GetPeriodsTotal() < 2 || p.GetPeriodsHit() < 1 {
		t.Fatalf("recurrence=%v progress=%v", g.Msg.GetGoal().GetRecurrence(), p)
	}

	_, err = goals.CreateGoal(ctx, connect.NewRequest(&fitterv1.CreateGoalRequest{
		Name: "Daily forever", PeriodStart: ts(start), PeriodEnd: ts(start + 3*365*24*3600), Clauses: clause,
		Recurrence: fitterv1.Recurrence_RECURRENCE_DAILY,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("daily over 3y: got %v, want InvalidArgument", err)
	}

	bad := manual()
	bad.SportType = "Run"
	huge := int32(99999)
	bad.UtcOffsetMinutes = &huge
	if _, err := acts.CreateManualActivity(ctx, connect.NewRequest(&fitterv1.CreateManualActivityRequest{Activity: bad})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("offset 99999: got %v, want InvalidArgument", err)
	}
}
