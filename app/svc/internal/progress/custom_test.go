package progress_test

import (
	"testing"

	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/progress"
	"github.com/fitter/svc/internal/sports"
)

// TestEvaluate_CustomMetricClause is Task 4's core end-to-end proof: a goal
// clause referencing a custom metric's key evaluates correctly against a
// synthetic activity, using progress.Evaluate directly — the exact same
// evaluation engine every built-in metric already goes through. This
// exercises the Clause.Def escape hatch (see progress.go's resolveDef):
// the service layer would build this Def via metrics.NewCustomDef and a
// DB-backed lookup; here it's built by hand, which is exactly the point —
// the custom-Def mechanism doesn't need Task 5's manual-entry plumbing to
// be independently testable.
func TestEvaluate_CustomMetricClause(t *testing.T) {
	benchPress := metrics.NewCustomDef("bench_press", "Bench press", "kg", metrics.AggMax, true, false, true, true)

	g := progress.Goal{
		ID:          "g-custom",
		Sport:       sports.Selector{SportType: "powerlifting"}, // a custom sport key, exact-match branch
		PeriodStart: 1000,
		PeriodEnd:   2000,
		Clauses: []progress.Clause{
			{
				ID: "c1", Position: 0, Scope: progress.ScopeAggregate,
				Metric: "bench_press", Def: benchPress,
				Comparator: progress.GTE, Value: 90,
			},
		},
	}

	acts := []metrics.Activity{
		// Freestanding manual-entry-shaped synthetic activity: all
		// Strava-numeric fields zero, SportType set to the custom sport
		// key, value carried only in Custom.
		{
			StravaActivityID: 0,
			SportType:        "powerlifting",
			Type:             "powerlifting",
			StartDate:        1500,
			Custom:           map[metrics.Key]float64{"bench_press": 100},
		},
		// Out of period: must not count.
		{
			SportType: "powerlifting", Type: "powerlifting",
			StartDate: 500,
			Custom:    map[metrics.Key]float64{"bench_press": 999},
		},
		// Wrong sport: must not count even though it's in period and
		// carries the same custom key.
		{
			SportType: "Run", Type: "Run",
			StartDate: 1600,
			Custom:    map[metrics.Key]float64{"bench_press": 999},
		},
	}

	res := progress.Evaluate(g, acts)

	if res.ConsideredCount != 1 {
		t.Fatalf("ConsideredCount = %d, want 1 (sport+period filtering must apply to a custom metric exactly like a built-in one)", res.ConsideredCount)
	}
	cr := res.Clauses[0]
	if !cr.HasValue {
		t.Fatal("HasValue = false, want true")
	}
	if cr.CurrentValue != 100 {
		t.Fatalf("CurrentValue = %v, want 100", cr.CurrentValue)
	}
	if !cr.Satisfied {
		t.Fatal("clause Satisfied = false, want true (100 >= 90)")
	}
	if !res.Satisfied {
		t.Fatal("Result.Satisfied = false, want true")
	}
}

// TestEvaluate_CustomMetricClause_SingleScope confirms SINGLE-scope
// evaluation (qualifying/best-attempt selection) also works transparently
// for a custom metric.
func TestEvaluate_CustomMetricClause_SingleScope(t *testing.T) {
	benchPress := metrics.NewCustomDef("bench_press", "Bench press", "kg", metrics.AggMax, true, false, true, true)

	g := progress.Goal{
		ID:          "g-custom-single",
		PeriodStart: 0,
		PeriodEnd:   10000,
		Clauses: []progress.Clause{
			{ID: "c1", Position: 0, Scope: progress.ScopeSingle, Metric: "bench_press", Def: benchPress, Comparator: progress.GTE, Value: 100},
		},
	}

	acts := []metrics.Activity{
		{StravaActivityID: 1, StartDate: 100, Custom: map[metrics.Key]float64{"bench_press": 80}},
		{StravaActivityID: 2, StartDate: 200, Custom: map[metrics.Key]float64{"bench_press": 105}},
	}

	res := progress.Evaluate(g, acts)
	if res.QualifyingActivityID != 2 {
		t.Fatalf("QualifyingActivityID = %d, want 2", res.QualifyingActivityID)
	}
	if !res.Satisfied {
		t.Fatal("Result.Satisfied = false, want true")
	}
}

// TestEvaluate_UnresolvedClauseFailsClosed confirms the pre-existing
// defensive behavior is unchanged when neither Def nor a registered Metric
// key resolves: the clause reports unsatisfied and valueless rather than
// panicking, exactly as it did before Clause.Def existed.
func TestEvaluate_UnresolvedClauseFailsClosed(t *testing.T) {
	g := progress.Goal{
		ID:          "g-unresolved",
		PeriodStart: 0,
		PeriodEnd:   10000,
		Clauses: []progress.Clause{
			{ID: "c1", Position: 0, Scope: progress.ScopeAggregate, Metric: "totally_unknown", Comparator: progress.GTE, Value: 1},
		},
	}
	res := progress.Evaluate(g, []metrics.Activity{{StartDate: 100}})
	if res.Satisfied {
		t.Fatal("Result.Satisfied = true, want false for an unresolved metric")
	}
	if res.Clauses[0].HasValue {
		t.Fatal("HasValue = true, want false for an unresolved metric")
	}
}
