package progress_test

import (
	"math"
	"testing"

	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/progress"
	"github.com/fitter/svc/internal/sports"
)

func ptr(f float64) *float64 { return &f }

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// act is a small builder for a metrics.Activity literal, keyed by the
// handful of fields each test cares about; StartDate/DistanceM/MovingTimeS
// etc. are set directly on the returned struct by the caller when needed.
func act(id int64, sportType string, start int64, distanceM, movingTimeS float64) metrics.Activity {
	return metrics.Activity{
		StravaActivityID: id,
		SportType:        sportType,
		Type:             sportType,
		StartDate:        start,
		DistanceM:        distanceM,
		MovingTimeS:      movingTimeS,
	}
}

// --- Case 1: volume goal, AGGREGATE distance GTE 100000, sport+period filtering ---

func TestVolumeGoal_AggregateDistanceSum(t *testing.T) {
	g := progress.Goal{
		ID:          "g1",
		Sport:       sports.Selector{Group: sports.GroupRun},
		PeriodStart: 1000,
		PeriodEnd:   2000,
		Clauses: []progress.Clause{
			{ID: "c1", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.Distance, Comparator: progress.GTE, Value: 100000},
		},
	}

	acts := []metrics.Activity{
		act(1, "Run", 1200, 40000, 3600),   // in scope
		act(2, "Run", 1500, 40000, 3600),   // in scope
		act(3, "Run", 1800, 25000, 3600),   // in scope, brings total to 105000
		act(4, "Run", 500, 999999, 3600),   // out of period (before start)
		act(5, "Run", 2500, 999999, 3600),  // out of period (after end)
		act(6, "Ride", 1300, 999999, 3600), // wrong sport
	}

	res := progress.Evaluate(g, acts)

	if res.ConsideredCount != 3 {
		t.Fatalf("ConsideredCount = %d, want 3", res.ConsideredCount)
	}
	cr := res.Clauses[0]
	if !almostEqual(cr.CurrentValue, 105000, 0.001) {
		t.Fatalf("CurrentValue = %v, want 105000", cr.CurrentValue)
	}
	if !cr.HasValue {
		t.Fatal("HasValue = false, want true")
	}
	if !cr.Satisfied {
		t.Fatal("clause Satisfied = false, want true")
	}
	if !res.Satisfied {
		t.Fatal("Result.Satisfied = false, want true")
	}
	if res.Ratio != 1 {
		t.Fatalf("Ratio = %v, want 1", res.Ratio)
	}
}

// --- Case 2: the core negative SINGLE-clause case ---

func TestPerformanceGoal_NoSingleActivitySatisfiesBoth(t *testing.T) {
	g := progress.Goal{
		ID:          "g2",
		PeriodStart: 0,
		PeriodEnd:   10000,
		Clauses: []progress.Clause{
			{ID: "dist", Position: 0, Scope: progress.ScopeSingle, Metric: metrics.Distance, Comparator: progress.GTE, Value: 10000},
			{ID: "time", Position: 1, Scope: progress.ScopeSingle, Metric: metrics.MovingTime, Comparator: progress.LTE, Value: 2700},
		},
	}

	// Activity 1: satisfies distance (12km) but not time (50 min = 3000s).
	a1 := act(1, "Run", 1000, 12000, 3000)
	// Activity 2: satisfies time (44 min = 2640s) but not distance (8km).
	a2 := act(2, "Run", 2000, 8000, 2640)

	acts := []metrics.Activity{a1, a2}

	res := progress.Evaluate(g, acts)

	if res.Satisfied {
		t.Fatal("Result.Satisfied = true, want false — no single activity satisfies both clauses")
	}
	if res.QualifyingActivityID != 0 {
		t.Fatalf("QualifyingActivityID = %d, want 0", res.QualifyingActivityID)
	}
	if res.BestAttemptActivityID == 0 {
		t.Fatal("BestAttemptActivityID = 0, want a non-zero best-attempt activity")
	}
	if len(res.Clauses) != 2 {
		t.Fatalf("len(Clauses) = %d, want 2", len(res.Clauses))
	}
	for _, cr := range res.Clauses {
		if cr.Satisfied {
			t.Fatalf("clause %s Satisfied = true, want false (best-attempt clauses are never satisfied)", cr.ClauseID)
		}
	}

	// Both clause results must come from the SAME best-attempt activity.
	// a1 satisfies exactly 1 clause (distance), a2 satisfies exactly 1
	// clause (time). Tie on satisfiedCount=1 each, so the tie-break is
	// mean ratio, then recency.
	//
	// a1: distance ratio = clamp(12000/10000,0,1) = 1; time ratio =
	//     clamp(2700/3000,0,1) = 0.9. mean = 0.95.
	// a2: distance ratio = clamp(8000/10000,0,1) = 0.8; time ratio =
	//     clamp(2700/2640,0,1) = 1. mean = 0.9.
	// a1's mean ratio (0.95) > a2's (0.9), so a1 must be the best attempt.
	if res.BestAttemptActivityID != 1 {
		t.Fatalf("BestAttemptActivityID = %d, want 1 (highest mean ratio)", res.BestAttemptActivityID)
	}

	byID := map[string]progress.ClauseResult{}
	for _, cr := range res.Clauses {
		byID[cr.ClauseID] = cr
	}
	distCR := byID["dist"]
	timeCR := byID["time"]
	if !almostEqual(distCR.CurrentValue, 12000, 0.001) {
		t.Fatalf("dist CurrentValue = %v, want 12000 (from activity 1)", distCR.CurrentValue)
	}
	if !almostEqual(timeCR.CurrentValue, 3000, 0.001) {
		t.Fatalf("time CurrentValue = %v, want 3000 (from activity 1, NOT activity 2's 2640)", timeCR.CurrentValue)
	}
}

// --- Case 3: same fixture plus one activity satisfying both ---

func TestPerformanceGoal_OneActivitySatisfiesBoth(t *testing.T) {
	g := progress.Goal{
		ID:          "g3",
		PeriodStart: 0,
		PeriodEnd:   10000,
		Clauses: []progress.Clause{
			{ID: "dist", Position: 0, Scope: progress.ScopeSingle, Metric: metrics.Distance, Comparator: progress.GTE, Value: 10000},
			{ID: "time", Position: 1, Scope: progress.ScopeSingle, Metric: metrics.MovingTime, Comparator: progress.LTE, Value: 2700},
		},
	}

	a1 := act(1, "Run", 1000, 12000, 3000) // distance only
	a2 := act(2, "Run", 2000, 8000, 2640)  // time only
	a3 := act(3, "Run", 3000, 11000, 2500) // both: 11km, 41:40

	acts := []metrics.Activity{a1, a2, a3}
	res := progress.Evaluate(g, acts)

	if !res.Satisfied {
		t.Fatal("Result.Satisfied = false, want true")
	}
	if res.QualifyingActivityID != 3 {
		t.Fatalf("QualifyingActivityID = %d, want 3", res.QualifyingActivityID)
	}
	if res.BestAttemptActivityID != 0 {
		t.Fatalf("BestAttemptActivityID = %d, want 0 (a qualifying activity exists)", res.BestAttemptActivityID)
	}
	for _, cr := range res.Clauses {
		if !cr.Satisfied {
			t.Fatalf("clause %s Satisfied = false, want true", cr.ClauseID)
		}
	}
}

// --- Case 4: hybrid AGGREGATE + SINGLE ---

func TestHybridGoal_AggregateAndSingle(t *testing.T) {
	g := progress.Goal{
		ID:          "g4",
		PeriodStart: 0,
		PeriodEnd:   10000,
		Clauses: []progress.Clause{
			{ID: "vol", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.Distance, Comparator: progress.GTE, Value: 20000},
			{ID: "single", Position: 1, Scope: progress.ScopeSingle, Metric: metrics.Distance, Comparator: progress.GTE, Value: 10000},
		},
	}

	// Enough distance in aggregate (sum 21000 >= 20000), but no single
	// activity individually reaches 10000.
	partial := []metrics.Activity{
		act(1, "Run", 1000, 7000, 3000),
		act(2, "Run", 2000, 7000, 3000),
		act(3, "Run", 3000, 7000, 3000),
	}
	res := progress.Evaluate(g, partial)
	if res.Satisfied {
		t.Fatal("Result.Satisfied = true, want false — no activity satisfies the SINGLE clause")
	}

	// Now with an activity that also satisfies the SINGLE clause.
	full := []metrics.Activity{
		act(1, "Run", 1000, 8000, 3000),
		act(2, "Run", 2000, 12000, 3000),
	}
	res2 := progress.Evaluate(g, full)
	if !res2.Satisfied {
		t.Fatal("Result.Satisfied = false, want true — aggregate and single both hold")
	}
}

// --- Case 5: nullable metric, partial data ---

func TestNullableMetric_PartialData(t *testing.T) {
	g := progress.Goal{
		ID:          "g5",
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			{ID: "hr", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.AverageHeartrate, Comparator: progress.GTE, Value: 140},
		},
	}

	var acts []metrics.Activity
	var wantWeightedSum, wantWeightSum float64
	for i := 0; i < 20; i++ {
		a := act(int64(i+1), "Run", int64(1000+i), 5000, 1800) // 30 min each
		if i < 12 {
			hr := 130.0 + float64(i) // varying HR values, all set
			a.AverageHeartrate = ptr(hr)
			wantWeightedSum += hr * a.MovingTimeS
			wantWeightSum += a.MovingTimeS
		}
		acts = append(acts, a)
	}

	res := progress.Evaluate(g, acts)
	if res.ConsideredCount != 20 {
		t.Fatalf("ConsideredCount = %d, want 20", res.ConsideredCount)
	}
	cr := res.Clauses[0]
	if cr.CountedCount != 12 {
		t.Fatalf("CountedCount = %d, want 12", cr.CountedCount)
	}
	if !cr.HasValue {
		t.Fatal("HasValue = false, want true")
	}
	wantMean := wantWeightedSum / wantWeightSum
	if !almostEqual(cr.CurrentValue, wantMean, 0.0001) {
		t.Fatalf("CurrentValue = %v, want %v (duration-weighted mean of the 12)", cr.CurrentValue, wantMean)
	}
}

// --- Case 6: nullable metric, zero data ---

func TestNullableMetric_ZeroData(t *testing.T) {
	g := progress.Goal{
		ID:          "g6",
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			{ID: "hr", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.AverageHeartrate, Comparator: progress.LTE, Value: 150},
		},
	}

	acts := []metrics.Activity{
		act(1, "Run", 1000, 5000, 1800),
		act(2, "Run", 1100, 5000, 1800),
	}

	res := progress.Evaluate(g, acts)
	cr := res.Clauses[0]
	if cr.HasValue {
		t.Fatal("HasValue = true, want false — no activity carries average_heartrate")
	}
	if cr.Satisfied {
		t.Fatal("clause Satisfied = true, want false — must not be vacuously true")
	}
	if cr.Ratio != 0 {
		t.Fatalf("Ratio = %v, want 0", cr.Ratio)
	}
	if res.Satisfied {
		t.Fatal("Result.Satisfied = true, want false")
	}
}

// --- Case 7: duration-weighted average vs naive mean ---

func TestDurationWeightedAverage_NotNaiveMean(t *testing.T) {
	g := progress.Goal{
		ID:          "g7",
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			{ID: "spd", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.AverageSpeed, Comparator: progress.GTE, Value: 0},
		},
	}

	acts := []metrics.Activity{
		func() metrics.Activity {
			a := act(1, "Run", 1000, 10800, 3600) // 3.0 m/s for 3600s
			a.AverageSpeedMS = 3.0
			return a
		}(),
		func() metrics.Activity {
			a := act(2, "Run", 5000, 3000, 600) // 5.0 m/s for 600s
			a.AverageSpeedMS = 5.0
			return a
		}(),
	}

	res := progress.Evaluate(g, acts)
	cr := res.Clauses[0]

	// weighted = (3.0*3600 + 5.0*600) / (3600+600) = (10800+3000)/4200 = 13800/4200 = 3.2857142857
	wantWeighted := (3.0*3600 + 5.0*600) / (3600 + 600)
	if !almostEqual(wantWeighted, 3.286, 0.001) {
		t.Fatalf("sanity: expected weighted const ~3.286, computed %v", wantWeighted)
	}
	if !almostEqual(cr.CurrentValue, 3.286, 0.001) {
		t.Fatalf("CurrentValue = %v, want ≈3.286 (duration-weighted)", cr.CurrentValue)
	}
	naiveMean := (3.0 + 5.0) / 2
	if almostEqual(cr.CurrentValue, naiveMean, 0.01) {
		t.Fatalf("CurrentValue = %v, must NOT equal naive mean %v", cr.CurrentValue, naiveMean)
	}
}

// --- Case 8: MAX aggregation, mix of present/absent ---

func TestMaxAggregation_PresentAndAbsent(t *testing.T) {
	g := progress.Goal{
		ID:          "g8",
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			{ID: "maxhr", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.MaxHeartrate, Comparator: progress.GTE, Value: 180},
		},
	}

	a1 := act(1, "Run", 1000, 5000, 1800)
	a1.MaxHeartrate = ptr(175)
	a2 := act(2, "Run", 1100, 5000, 1800) // no HR
	a3 := act(3, "Run", 1200, 5000, 1800)
	a3.MaxHeartrate = ptr(190)
	a4 := act(4, "Run", 1300, 5000, 1800) // no HR

	res := progress.Evaluate(g, []metrics.Activity{a1, a2, a3, a4})
	cr := res.Clauses[0]
	if !cr.HasValue {
		t.Fatal("HasValue = false, want true")
	}
	if !almostEqual(cr.CurrentValue, 190, 0.001) {
		t.Fatalf("CurrentValue = %v, want 190 (max of 175, 190)", cr.CurrentValue)
	}
	if cr.CountedCount != 2 {
		t.Fatalf("CountedCount = %d, want 2", cr.CountedCount)
	}
	if !cr.Satisfied {
		t.Fatal("clause Satisfied = false, want true")
	}
}

// --- Case 9: pace as Σtime/Σdistance, not mean of per-activity paces ---

func TestPaceAggregation_NotMeanOfPerActivityPaces(t *testing.T) {
	g := progress.Goal{
		ID:          "g9",
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			{ID: "pace", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.Pace, Comparator: progress.LTE, Value: 999999},
		},
	}

	// Run 1: 1km in 300s -> pace 300 s/km.
	// Run 2: 20km in 4800s -> pace 240 s/km.
	// Mean of per-activity paces = (300+240)/2 = 270.
	// Aggregate pace = (300+4800)/((1000+20000)/1000) = 5100/21 ≈ 242.857.
	a1 := act(1, "Run", 1000, 1000, 300)
	a2 := act(2, "Run", 2000, 20000, 4800)

	res := progress.Evaluate(g, []metrics.Activity{a1, a2})
	cr := res.Clauses[0]
	if !cr.HasValue {
		t.Fatal("HasValue = false, want true")
	}
	wantPace := 5100.0 / 21.0
	if !almostEqual(cr.CurrentValue, wantPace, 0.001) {
		t.Fatalf("CurrentValue = %v, want %v (Σtime/Σdistance)", cr.CurrentValue, wantPace)
	}
	naiveMean := (300.0 + 240.0) / 2
	if almostEqual(cr.CurrentValue, naiveMean, 0.5) {
		t.Fatalf("CurrentValue = %v, must NOT equal mean-of-paces %v", cr.CurrentValue, naiveMean)
	}

	// Zero total distance -> HasValue == false.
	zeroDist := act(3, "Run", 1000, 0, 300)
	res2 := progress.Evaluate(g, []metrics.Activity{zeroDist})
	if res2.Clauses[0].HasValue {
		t.Fatal("HasValue = true, want false — zero total distance makes pace undefined")
	}
}

// --- Case 10: activity_count is never nullable, satisfied over empty set ---

func TestActivityCount_EmptySetSatisfiesLTE(t *testing.T) {
	g := progress.Goal{
		ID:          "g10",
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			{ID: "cnt", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.ActivityCount, Comparator: progress.LTE, Value: 10},
		},
	}

	res := progress.Evaluate(g, nil)
	cr := res.Clauses[0]
	if !cr.HasValue {
		t.Fatal("HasValue = false, want true — activity_count is never nullable")
	}
	if cr.CurrentValue != 0 {
		t.Fatalf("CurrentValue = %v, want 0", cr.CurrentValue)
	}
	if !cr.Satisfied {
		t.Fatal("clause Satisfied = false, want true")
	}
	if !res.Satisfied {
		t.Fatal("Result.Satisfied = false, want true")
	}
}

// --- Case 11: sport selection via sports.Matches ---

func TestSportSelection(t *testing.T) {
	g := progress.Goal{
		ID:          "g11",
		Sport:       sports.Selector{Group: sports.GroupRun},
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			{ID: "cnt", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.ActivityCount, Comparator: progress.GTE, Value: 0},
		},
	}

	acts := []metrics.Activity{
		act(1, "Run", 1000, 1000, 300),
		act(2, "TrailRun", 1000, 1000, 300),
		act(3, "VirtualRun", 1000, 1000, 300),
		act(4, "TrackRun", 1000, 1000, 300),
		act(5, "Ride", 1000, 1000, 300), // excluded
		act(6, "Swim", 1000, 1000, 300), // excluded
	}
	res := progress.Evaluate(g, acts)
	if res.ConsideredCount != 4 {
		t.Fatalf("group selector: ConsideredCount = %d, want 4", res.ConsideredCount)
	}

	// Exact selector matches only that literal.
	gExact := g
	gExact.Sport = sports.Selector{SportType: "Run"}
	resExact := progress.Evaluate(gExact, acts)
	if resExact.ConsideredCount != 1 {
		t.Fatalf("exact selector: ConsideredCount = %d, want 1", resExact.ConsideredCount)
	}

	// Zero-value selector matches everything.
	gAll := g
	gAll.Sport = sports.Selector{}
	resAll := progress.Evaluate(gAll, acts)
	if resAll.ConsideredCount != 6 {
		t.Fatalf("zero-value selector: ConsideredCount = %d, want 6", resAll.ConsideredCount)
	}
}

// --- Case 12: period boundaries inclusive both ends ---

func TestPeriodBoundariesInclusive(t *testing.T) {
	g := progress.Goal{
		ID:          "g12",
		PeriodStart: 1000,
		PeriodEnd:   2000,
		Clauses: []progress.Clause{
			{ID: "cnt", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.ActivityCount, Comparator: progress.GTE, Value: 0},
		},
	}
	acts := []metrics.Activity{
		act(1, "Run", 1000, 1000, 300), // exactly on start
		act(2, "Run", 2000, 1000, 300), // exactly on end
		act(3, "Run", 999, 1000, 300),  // just before start
		act(4, "Run", 2001, 1000, 300), // just after end
	}
	res := progress.Evaluate(g, acts)
	if res.ConsideredCount != 2 {
		t.Fatalf("ConsideredCount = %d, want 2 (both boundary activities included)", res.ConsideredCount)
	}
}

// --- Case 13: ratio direction for GTE and LTE, including LTE current==0 ---

func TestRatioDirection(t *testing.T) {
	g := progress.Goal{
		ID:          "g13",
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			{ID: "gte", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.Distance, Comparator: progress.GTE, Value: 10000},
		},
	}
	// GTE: half of target.
	res := progress.Evaluate(g, []metrics.Activity{act(1, "Run", 1000, 5000, 300)})
	if !almostEqual(res.Clauses[0].Ratio, 0.5, 0.001) {
		t.Fatalf("GTE ratio = %v, want 0.5", res.Clauses[0].Ratio)
	}

	// GTE: exceeding target clamps to 1.
	res2 := progress.Evaluate(g, []metrics.Activity{act(1, "Run", 1000, 20000, 300)})
	if res2.Clauses[0].Ratio != 1 {
		t.Fatalf("GTE ratio (exceeding) = %v, want 1", res2.Clauses[0].Ratio)
	}

	// LTE: current > 0.
	gLTE := progress.Goal{
		ID:          "g13b",
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			{ID: "lte", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.MovingTime, Comparator: progress.LTE, Value: 1000},
		},
	}
	resLTE := progress.Evaluate(gLTE, []metrics.Activity{act(1, "Run", 1000, 5000, 2000)})
	if !almostEqual(resLTE.Clauses[0].Ratio, 0.5, 0.001) {
		t.Fatalf("LTE ratio = %v, want 0.5 (target/current = 1000/2000)", resLTE.Clauses[0].Ratio)
	}

	// LTE with current == 0 and HasValue -> ratio 1 (moving_time sum of a
	// single zero-duration activity legitimately sums to 0, non-nullable).
	resLTEZero := progress.Evaluate(gLTE, []metrics.Activity{act(1, "Run", 1000, 5000, 0)})
	if !resLTEZero.Clauses[0].HasValue {
		t.Fatal("HasValue = false, want true (moving_time is non-nullable)")
	}
	if resLTEZero.Clauses[0].Ratio != 1 {
		t.Fatalf("LTE ratio (current==0) = %v, want 1", resLTEZero.Clauses[0].Ratio)
	}
}

// --- Case 14: determinism, most recent candidate chosen ---

func TestDeterminism_MostRecentCandidateChosen(t *testing.T) {
	g := progress.Goal{
		ID:          "g14",
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			{ID: "dist", Position: 0, Scope: progress.ScopeSingle, Metric: metrics.Distance, Comparator: progress.GTE, Value: 5000},
		},
	}
	// Both candidates qualify; activity 2 started later.
	a1 := act(1, "Run", 1000, 6000, 1800)
	a2 := act(2, "Run", 2000, 7000, 1800)
	res := progress.Evaluate(g, []metrics.Activity{a1, a2})
	if res.QualifyingActivityID != 2 {
		t.Fatalf("QualifyingActivityID = %d, want 2 (most recent StartDate)", res.QualifyingActivityID)
	}

	// Same StartDate, tie-break on StravaActivityID desc.
	a3 := act(3, "Run", 5000, 6000, 1800)
	a4 := act(4, "Run", 5000, 7000, 1800)
	res2 := progress.Evaluate(g, []metrics.Activity{a3, a4})
	if res2.QualifyingActivityID != 4 {
		t.Fatalf("QualifyingActivityID = %d, want 4 (tie-break on StravaActivityID desc)", res2.QualifyingActivityID)
	}
}

// --- Case 15b: unrecognized Scope (including the zero value) fails closed ---

func TestUnrecognizedScope_FailsClosed(t *testing.T) {
	g := progress.Goal{
		ID:          "g15b",
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			// Scope left at its zero value: not ScopeAggregate, not
			// ScopeSingle. This is reachable from an unspecified proto enum
			// once Task 5/7 wire this up, and must never be silently
			// dropped — see progress.Evaluate's Scope switch.
			{ID: "bad-scope", Position: 0, Scope: progress.Scope(0), Metric: metrics.Distance, Comparator: progress.GTE, Value: 10000},
		},
	}

	res := progress.Evaluate(g, []metrics.Activity{act(1, "Run", 1000, 999999, 3600)})

	if res.Satisfied {
		t.Fatal("Result.Satisfied = true, want false — an unrecognized Scope must fail closed, not be silently dropped")
	}
	if len(res.Clauses) != 1 {
		t.Fatalf("len(Clauses) = %d, want 1 (every clause, recognized or not, produces exactly one ClauseResult)", len(res.Clauses))
	}
	cr := res.Clauses[0]
	if cr.ClauseID != "bad-scope" {
		t.Fatalf("Clauses[0].ClauseID = %q, want %q", cr.ClauseID, "bad-scope")
	}
	if cr.Position != 0 {
		t.Fatalf("Clauses[0].Position = %d, want 0", cr.Position)
	}
	if cr.Satisfied {
		t.Fatal("clause Satisfied = true, want false")
	}
	if res.Ratio != 0 {
		t.Fatalf("Result.Ratio = %v, want 0 (the unrecognized-scope clause's ratio must count toward the min)", res.Ratio)
	}
}

// --- Case 15: empty activity slice overall ---

func TestEmptyActivitySlice(t *testing.T) {
	g := progress.Goal{
		ID:          "g15",
		PeriodStart: 0,
		PeriodEnd:   100000,
		Clauses: []progress.Clause{
			{ID: "vol", Position: 0, Scope: progress.ScopeAggregate, Metric: metrics.Distance, Comparator: progress.GTE, Value: 10000},
			{ID: "single", Position: 1, Scope: progress.ScopeSingle, Metric: metrics.Distance, Comparator: progress.GTE, Value: 10000},
		},
	}

	res := progress.Evaluate(g, nil)
	if res.Satisfied {
		t.Fatal("Result.Satisfied = true, want false")
	}
	if res.ConsideredCount != 0 {
		t.Fatalf("ConsideredCount = %d, want 0", res.ConsideredCount)
	}
	if res.QualifyingActivityID != 0 {
		t.Fatalf("QualifyingActivityID = %d, want 0", res.QualifyingActivityID)
	}
	if res.BestAttemptActivityID != 0 {
		t.Fatalf("BestAttemptActivityID = %d, want 0", res.BestAttemptActivityID)
	}
	for _, cr := range res.Clauses {
		if cr.Satisfied {
			t.Fatalf("clause %s Satisfied = true, want false", cr.ClauseID)
		}
		if cr.Ratio != 0 {
			t.Fatalf("clause %s Ratio = %v, want 0", cr.ClauseID, cr.Ratio)
		}
	}
	if res.Ratio != 0 {
		t.Fatalf("Result.Ratio = %v, want 0", res.Ratio)
	}
}
