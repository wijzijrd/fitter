package progress_test

import (
	"testing"

	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/progress"
	"github.com/fitter/svc/internal/sports"
)

func windowGoal(metric metrics.Key, scope progress.Scope, value float64) progress.Goal {
	return progress.Goal{
		ID: "g", Sport: sports.Selector{}, PeriodStart: day(1, 0), PeriodEnd: day(29, 0) - 1, // 28 days
		Clauses: []progress.Clause{{ID: "c", Scope: scope, Metric: metric, Comparator: progress.GTE, Value: value}},
	}
}

func TestForWindow_SumTargetsArePaceProRated(t *testing.T) {
	g := windowGoal(metrics.Distance, progress.ScopeAggregate, 280000) // 280 km over 28 days
	wg, ok := progress.ForWindow(g, progress.Window{Start: day(5, 0), End: day(12, 0) - 1})
	if !ok || !almostEqual(wg.Clauses[0].Value, 70000, 1) { // 7 of 28 days
		t.Fatalf("ok=%v value=%v, want 70000", ok, wg.Clauses[0].Value)
	}
	r := progress.Evaluate(wg, []metrics.Activity{act(1, "Run", day(6, 8), 35000, 100), act(2, "Run", day(20, 8), 99000, 100)})
	if !almostEqual(r.Ratio, 0.5, 1e-9) { // only the in-window 35 km counts
		t.Fatalf("ratio = %v, want 0.5", r.Ratio)
	}
}

func TestForWindow_ClipsAtCampaignEdgeAndMisses(t *testing.T) {
	g := windowGoal(metrics.Distance, progress.ScopeAggregate, 280000)
	wg, ok := progress.ForWindow(g, progress.Window{Start: day(26, 0), End: day(40, 0)})
	if !ok || wg.PeriodEnd != g.PeriodEnd || !almostEqual(wg.Clauses[0].Value, 280000*3.0/28, 1) {
		t.Fatalf("clipped partial window: ok=%v %+v", ok, wg)
	}
	if _, ok := progress.ForWindow(g, progress.Window{Start: day(30, 0), End: day(31, 0)}); ok {
		t.Fatal("window outside the period should not apply")
	}
}

func TestForWindow_MaxAndSingleTargetsStayWhole(t *testing.T) {
	g := windowGoal(metrics.MaxHeartrate, progress.ScopeAggregate, 180)
	wg, _ := progress.ForWindow(g, progress.Window{Start: day(5, 0), End: day(12, 0) - 1})
	if wg.Clauses[0].Value != 180 {
		t.Fatalf("max target pro-rated: %v", wg.Clauses[0].Value)
	}
	g = windowGoal(metrics.Distance, progress.ScopeSingle, 5000)
	if wg, _ = progress.ForWindow(g, progress.Window{Start: day(5, 0), End: day(12, 0) - 1}); wg.Clauses[0].Value != 5000 {
		t.Fatalf("single target pro-rated: %v", wg.Clauses[0].Value)
	}
}

func TestWindowExcluded(t *testing.T) {
	empty := progress.Result{}
	if !progress.WindowExcluded(windowGoal(metrics.MaxWeightKg, progress.ScopeAggregate, 100), empty) {
		t.Error("max goal with no data should be excluded")
	}
	if !progress.WindowExcluded(windowGoal(metrics.Distance, progress.ScopeSingle, 5000), empty) {
		t.Error("single goal with no data should be excluded")
	}
	if progress.WindowExcluded(windowGoal(metrics.Distance, progress.ScopeAggregate, 5000), empty) {
		t.Error("sum goal with no data stays, at zero")
	}
	rec := windowGoal(metrics.MaxWeightKg, progress.ScopeAggregate, 100)
	rec.Recurrence = progress.RecurWeekly
	if progress.WindowExcluded(rec, empty) {
		t.Error("recurring goal stays")
	}
	if progress.WindowExcluded(windowGoal(metrics.MaxWeightKg, progress.ScopeAggregate, 100), progress.Result{ConsideredCount: 1}) {
		t.Error("goal with activity stays")
	}
}
