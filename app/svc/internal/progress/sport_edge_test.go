package progress_test

import (
	"testing"

	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/progress"
	"github.com/fitter/svc/internal/sports"
)

func distanceGoal(sel sports.Selector, targetM float64) progress.Goal {
	return progress.Goal{
		ID: "g", Sport: sel, PeriodStart: 0, PeriodEnd: 100000,
		Clauses: []progress.Clause{{ID: "c", Scope: progress.ScopeAggregate, Metric: metrics.Distance, Comparator: progress.GTE, Value: targetM}},
	}
}

// Every curated sport type, as a real activity, is counted by a goal on its group and by no
// goal on any other group; "all sports" counts everything.
func TestEveryCuratedSportCountsOnlyTowardItsOwnGroup(t *testing.T) {
	var id int64
	for _, owner := range sports.Groups() {
		for _, st := range owner.SportTypes {
			id++
			a := []metrics.Activity{act(id, st, 1000, 1000, 600)}
			for _, sel := range sports.Groups() {
				res := progress.Evaluate(distanceGoal(sports.Selector{Group: sel.Group}, 1000), a)
				want := 0
				if sel.Group == owner.Group {
					want = 1
				}
				if res.ConsideredCount != want {
					t.Errorf("%s activity vs %q goal: considered %d, want %d", st, sel.Group, res.ConsideredCount, want)
				}
			}
			if res := progress.Evaluate(distanceGoal(sports.Selector{}, 1000), a); res.ConsideredCount != 1 || !res.Satisfied {
				t.Errorf("%s activity vs all-sports goal: %+v", st, res)
			}
		}
	}
}

func TestUnknownAndEmptySportsLandInOther(t *testing.T) {
	acts := []metrics.Activity{
		act(1, "Zorb", 1000, 1000, 60),
		act(2, "", 1000, 1000, 60), // seeded row with neither field
		act(3, "Run", 1000, 1000, 60),
	}
	if res := progress.Evaluate(distanceGoal(sports.Selector{Group: sports.GroupOther}, 1), acts); res.ConsideredCount != 2 {
		t.Fatalf("other goal considered %d, want the 2 unmapped activities", res.ConsideredCount)
	}
}

// A custom sport only ever matches through an exact-type selector on its key.
func TestCustomSportKeyMatchesOnlyItsOwnActivities(t *testing.T) {
	acts := []metrics.Activity{
		act(1, "powerlifting", 1000, 0, 3600),
		act(2, "powerlifting", 2000, 0, 1800),
		act(3, "WeightTraining", 1000, 0, 3600),
	}
	g := progress.Goal{
		ID: "g", Sport: sports.Selector{SportType: "powerlifting"}, PeriodStart: 0, PeriodEnd: 100000,
		Clauses: []progress.Clause{{ID: "c", Scope: progress.ScopeAggregate, Metric: metrics.ActivityCount, Comparator: progress.GTE, Value: 2}},
	}
	res := progress.Evaluate(g, acts)
	if res.ConsideredCount != 2 || !res.Satisfied {
		t.Fatalf("custom-sport goal: %+v, want 2 considered and satisfied", res)
	}
	// Its activities are not Strength's: they resolve to "other" for group goals.
	if res := progress.Evaluate(distanceGoal(sports.Selector{Group: sports.GroupStrength}, 1), acts); res.ConsideredCount != 1 {
		t.Fatalf("strength group counted %d, want only the WeightTraining one", res.ConsideredCount)
	}
}

// HIIT, Crossfit and Workout left Strength for HIIT & circuits (a deliberate regrouping).
func TestRegroupedSportsStopCountingTowardStrength(t *testing.T) {
	acts := []metrics.Activity{
		act(1, "WeightTraining", 1000, 0, 60), act(2, "Crossfit", 1000, 0, 60),
		act(3, "Workout", 1000, 0, 60), act(4, "HighIntensityIntervalTraining", 1000, 0, 60),
	}
	count := func(g sports.Group) int {
		return progress.Evaluate(distanceGoal(sports.Selector{Group: g}, 1), acts).ConsideredCount
	}
	if s, c := count(sports.GroupStrength), count(sports.GroupCircuits); s != 1 || c != 3 {
		t.Fatalf("strength counted %d (want 1), circuits counted %d (want 3)", s, c)
	}
}

// --- Over 100% -------------------------------------------------------------------------

// Progress caps at 100%, but the underlying value keeps counting, so the UI can still show
// "130 km of 100 km". Anything that ranks by ratio alone can't tell these apart; the next
// test pins what the leaderboard does about it.
func TestOvershootCapsRatioButKeepsValue(t *testing.T) {
	for _, dist := range []float64{10000, 10001, 13000, 1e9} {
		res := progress.Evaluate(distanceGoal(sports.Selector{}, 10000), []metrics.Activity{act(1, "Run", 1000, dist, 600)})
		c := res.Clauses[0]
		if c.Ratio != 1 || !c.Satisfied || !res.Satisfied {
			t.Errorf("%.0f m vs 10 km: ratio %v satisfied %v, want exactly 1 and satisfied", dist, c.Ratio, c.Satisfied)
		}
		if c.CurrentValue != dist {
			t.Errorf("%.0f m: current value %v, want the true total", dist, c.CurrentValue)
		}
	}
}

func TestJustUnderTargetIsNotComplete(t *testing.T) {
	res := progress.Evaluate(distanceGoal(sports.Selector{}, 10000), []metrics.Activity{act(1, "Run", 1000, 9999.9, 600)})
	if res.Satisfied || res.Ratio >= 1 {
		t.Fatalf("9999.9 m vs 10 km: %+v, want not satisfied and ratio below 1", res)
	}
}

func TestZeroAndNegativeDistanceNeverGoBelowZeroPercent(t *testing.T) {
	for _, dist := range []float64{0, -5} {
		res := progress.Evaluate(distanceGoal(sports.Selector{}, 10000), []metrics.Activity{act(1, "Run", 1000, dist, 600)})
		if res.Ratio < 0 || res.Ratio > 1 {
			t.Errorf("distance %v: ratio %v escaped 0..1", dist, res.Ratio)
		}
	}
}

// Several activities add up past the target; the ratio still tops out at one.
func TestManyActivitiesAddPastTheTarget(t *testing.T) {
	acts := []metrics.Activity{act(1, "Run", 1000, 6000, 600), act(2, "Run", 2000, 6000, 600), act(3, "Run", 3000, 6000, 600)}
	res := progress.Evaluate(distanceGoal(sports.Selector{Group: sports.GroupRun}, 10000), acts)
	if res.Ratio != 1 || res.Clauses[0].CurrentValue != 18000 || res.SatisfiedAt != 2000 {
		t.Fatalf("got ratio %v value %v satisfiedAt %d, want 1, 18000 and the second activity (2000)", res.Ratio, res.Clauses[0].CurrentValue, res.SatisfiedAt)
	}
}
