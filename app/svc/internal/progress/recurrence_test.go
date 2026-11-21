package progress_test

import (
	"testing"
	"time"

	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/progress"
	"github.com/fitter/svc/internal/sports"
)

func day(d int, hour int) int64 { // January 2026; the 5th is a Monday
	return time.Date(2026, 1, d, hour, 0, 0, 0, time.UTC).Unix()
}

func activeDaysGoal(rec progress.Recurrence, start, end, now int64, min float64) progress.Goal {
	return progress.Goal{
		ID: "g", Sport: sports.Selector{}, PeriodStart: start, PeriodEnd: end, Recurrence: rec, Now: now,
		Clauses: []progress.Clause{{ID: "c", Scope: progress.ScopeAggregate, Metric: metrics.ActiveDays, Comparator: progress.GTE, Value: min}},
	}
}

func run(id int64, d, h int) metrics.Activity { return act(id, "Run", day(d, h), 1000, 600) }

func TestWeeklyHabit_RatioIsBucketsHitOverElapsed(t *testing.T) {
	acts := []metrics.Activity{
		run(1, 5, 8), run(2, 7, 8), // week 1: 2 days  -> hit
		run(3, 13, 8),                // week 2: 1 day -> miss
		run(4, 19, 8), run(5, 20, 8), // week 3: 2 days -> hit
	}
	// Period over: 3 weeks, 2 hit.
	g := activeDaysGoal(progress.RecurWeekly, day(5, 0), day(25, 23), 0, 2)
	r := progress.Evaluate(g, acts)
	if r.PeriodsHit != 2 || r.PeriodsElapsed != 3 || r.PeriodsTotal != 3 || r.Satisfied || !almostEqual(r.Ratio, 2.0/3, 1e-9) {
		t.Fatalf("got hit=%d elapsed=%d total=%d sat=%v ratio=%v", r.PeriodsHit, r.PeriodsElapsed, r.PeriodsTotal, r.Satisfied, r.Ratio)
	}

	// Mid week 3, not yet hit: running bucket doesn't drag the score down.
	g.Now = day(21, 12)
	r = progress.Evaluate(g, acts[:3])
	if r.PeriodsHit != 1 || r.PeriodsElapsed != 2 || !almostEqual(r.Ratio, 0.5, 1e-9) {
		t.Fatalf("running bucket: hit=%d elapsed=%d ratio=%v", r.PeriodsHit, r.PeriodsElapsed, r.Ratio)
	}
	if len(r.Clauses) != 1 || r.Clauses[0].CurrentValue != 0 {
		t.Fatalf("clauses should be the running bucket's live progress: %+v", r.Clauses)
	}
}

func TestWeeklyHabit_AllHitIsSatisfied(t *testing.T) {
	acts := []metrics.Activity{run(1, 5, 8), run(2, 6, 8), run(3, 12, 8), run(4, 13, 8)}
	g := activeDaysGoal(progress.RecurWeekly, day(5, 0), day(18, 23), 0, 2)
	if r := progress.Evaluate(g, acts); !r.Satisfied || r.Ratio != 1 {
		t.Fatalf("want satisfied, got %+v", r)
	}
}

func TestWeekEdge_LocalTimeDecidesTheBucket(t *testing.T) {
	// Sunday 23:30 UTC, but 01:30 Monday locally -> belongs to the NEXT week.
	late := run(1, 11, 23)
	late.StartDateLocal = day(12, 1)
	g := activeDaysGoal(progress.RecurWeekly, day(5, 0), day(18, 23), 0, 1)
	r := progress.Evaluate(g, []metrics.Activity{late})
	if r.PeriodsHit != 1 {
		t.Fatalf("local Monday run should hit only week 2, got hit=%d", r.PeriodsHit)
	}
	late.StartDateLocal = 0 // unknown local time falls back to UTC (Sunday = week 1)
	if r = progress.Evaluate(g, []metrics.Activity{late}); r.PeriodsHit != 1 || r.Clauses[0].CurrentValue != 0 {
		t.Fatalf("UTC fallback: hit=%d", r.PeriodsHit)
	}
}

func TestPartialFirstWeek(t *testing.T) {
	// Period starts Wednesday; the Monday run is outside it and must not count.
	g := activeDaysGoal(progress.RecurWeekly, day(7, 0), day(11, 23), 0, 1)
	if n := progress.CountBuckets(progress.RecurWeekly, g.PeriodStart, g.PeriodEnd); n != 1 {
		t.Fatalf("buckets = %d, want 1", n)
	}
	r := progress.Evaluate(g, []metrics.Activity{run(1, 5, 8)})
	if r.PeriodsHit != 0 {
		t.Fatalf("activity before the period counted: %+v", r)
	}
	if r = progress.Evaluate(g, []metrics.Activity{run(2, 8, 8)}); r.PeriodsHit != 1 || !r.Satisfied {
		t.Fatalf("activity inside the period should hit: %+v", r)
	}
}

func TestDailyAndMonthly(t *testing.T) {
	g := activeDaysGoal(progress.RecurDaily, day(5, 0), day(7, 23), 0, 1)
	r := progress.Evaluate(g, []metrics.Activity{run(1, 5, 8), run(2, 7, 8)})
	if r.PeriodsTotal != 3 || r.PeriodsHit != 2 {
		t.Fatalf("daily: %+v", r)
	}
	feb := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC).Unix()
	if n := progress.CountBuckets(progress.RecurMonthly, day(20, 0), feb); n != 2 {
		t.Fatalf("monthly buckets = %d, want 2", n)
	}
}

func TestFreshGoalHasZeroRatio(t *testing.T) {
	g := activeDaysGoal(progress.RecurWeekly, day(5, 0), day(25, 23), day(5, 9), 2)
	r := progress.Evaluate(g, nil)
	if r.Ratio != 0 || r.PeriodsElapsed != 0 || r.Satisfied {
		t.Fatalf("%+v", r)
	}
}

func TestAbsurdPeriodsAreBounded(t *testing.T) {
	if n := progress.CountBuckets(progress.RecurDaily, 0, 1<<60); n != progress.MaxBuckets+1 {
		t.Fatalf("CountBuckets not capped: %d", n)
	}
	g := activeDaysGoal(progress.RecurDaily, 1<<40, 5, 0, 1) // inverted: must not panic
	_ = progress.Evaluate(g, nil)
}
