package metrics_test

import (
	"testing"

	"github.com/fitter/svc/internal/metrics"
)

// TestNewCustomDef_Sum mirrors sumAcc's own nullable-skip test: activities
// missing the custom key are skipped, present ones are summed, and a
// nullable custom metric with zero contributing activities reports ok=false
// (never a silent 0).
func TestNewCustomDef_Sum(t *testing.T) {
	def := metrics.NewCustomDef("bench_press_volume", "Bench press volume", "kg", metrics.AggSum, true, false, true, true)

	acc := def.NewAcc()
	acc.Add(metrics.Activity{Custom: map[metrics.Key]float64{"bench_press_volume": 100}})
	acc.Add(metrics.Activity{}) // no Custom map at all: must be skipped, not panic
	acc.Add(metrics.Activity{Custom: map[metrics.Key]float64{"other_key": 50}})
	acc.Add(metrics.Activity{Custom: map[metrics.Key]float64{"bench_press_volume": 40}})

	value, counted, ok := acc.Result()
	if !ok {
		t.Fatal("Result ok = false, want true (two contributing activities)")
	}
	if counted != 2 {
		t.Fatalf("counted = %d, want 2", counted)
	}
	if value != 140 {
		t.Fatalf("value = %v, want 140", value)
	}
}

// TestNewCustomDef_Sum_NoContribution confirms the nullable-skip contract:
// zero contributing activities on a nullable custom metric must report
// ok=false, exactly like every nullable built-in sumAcc.
func TestNewCustomDef_Sum_NoContribution(t *testing.T) {
	def := metrics.NewCustomDef("bench_press_volume", "Bench press volume", "kg", metrics.AggSum, true, false, true, true)
	acc := def.NewAcc()
	acc.Add(metrics.Activity{})
	acc.Add(metrics.Activity{Custom: map[metrics.Key]float64{"unrelated": 1}})

	_, counted, ok := acc.Result()
	if ok {
		t.Fatal("Result ok = true, want false (no activity ever carried the metric)")
	}
	if counted != 0 {
		t.Fatalf("counted = %d, want 0", counted)
	}
}

// TestNewCustomDef_Max confirms max aggregation only considers present
// values.
func TestNewCustomDef_Max(t *testing.T) {
	def := metrics.NewCustomDef("deadlift_max", "Deadlift max", "kg", metrics.AggMax, true, false, true, true)
	acc := def.NewAcc()
	acc.Add(metrics.Activity{Custom: map[metrics.Key]float64{"deadlift_max": 120}})
	acc.Add(metrics.Activity{})
	acc.Add(metrics.Activity{Custom: map[metrics.Key]float64{"deadlift_max": 150}})
	acc.Add(metrics.Activity{Custom: map[metrics.Key]float64{"deadlift_max": 90}})

	value, counted, ok := acc.Result()
	if !ok || counted != 3 || value != 150 {
		t.Fatalf("Result = (%v, %d, %v), want (150, 3, true)", value, counted, ok)
	}
}

// TestNewCustomDef_WeightedAverage confirms average aggregation is
// duration-weighted by the activity's MovingTimeS, exactly like every
// built-in weightedAvgAcc metric, and that a zero-moving-time activity
// contributes zero weight rather than dividing by zero.
func TestNewCustomDef_WeightedAverage(t *testing.T) {
	def := metrics.NewCustomDef("session_rpe", "Session RPE", "score", metrics.AggWeightedAvg, true, false, true, true)
	acc := def.NewAcc()
	acc.Add(metrics.Activity{MovingTimeS: 3600, Custom: map[metrics.Key]float64{"session_rpe": 6}})
	acc.Add(metrics.Activity{MovingTimeS: 0, Custom: map[metrics.Key]float64{"session_rpe": 10}}) // zero weight, must not skew result
	acc.Add(metrics.Activity{MovingTimeS: 1800, Custom: map[metrics.Key]float64{"session_rpe": 8}})

	value, counted, ok := acc.Result()
	if !ok {
		t.Fatal("Result ok = false, want true")
	}
	if counted != 2 {
		t.Fatalf("counted = %d, want 2 (zero-moving-time activity excluded)", counted)
	}
	want := (6*3600 + 8*1800) / (3600.0 + 1800.0)
	if value != want {
		t.Fatalf("value = %v, want %v", value, want)
	}
}

// TestNewCustomDef_Count confirms AggCount counts only activities where the
// custom metric was actually present — unlike the built-in ActivityCount,
// which counts every activity unconditionally. This distinction is the
// whole reason NewCustomDef uses its own presenceCountAcc instead of
// reusing countAcc.
func TestNewCustomDef_Count(t *testing.T) {
	def := metrics.NewCustomDef("workout_logged", "Workout logged", "count", metrics.AggCount, true, false, false, true)
	acc := def.NewAcc()
	acc.Add(metrics.Activity{Custom: map[metrics.Key]float64{"workout_logged": 1}})
	acc.Add(metrics.Activity{}) // no entry for this metric: must not count
	acc.Add(metrics.Activity{Custom: map[metrics.Key]float64{"workout_logged": 1}})

	value, counted, ok := acc.Result()
	if !ok {
		t.Fatal("Result ok = false, want true (count is always meaningful, even at zero)")
	}
	if counted != 2 || value != 2 {
		t.Fatalf("Result = (%v, %d), want (2, 2)", value, counted)
	}
}

// TestNewCustomDef_Single confirms Single reads the custom bag with the
// same (value, ok) shape every built-in extractor already returns.
func TestNewCustomDef_Single(t *testing.T) {
	def := metrics.NewCustomDef("bench_press", "Bench press", "kg", metrics.AggMax, true, false, true, true)

	v, ok := def.Single(metrics.Activity{Custom: map[metrics.Key]float64{"bench_press": 100}})
	if !ok || v != 100 {
		t.Fatalf("Single = (%v, %v), want (100, true)", v, ok)
	}

	v, ok = def.Single(metrics.Activity{})
	if ok {
		t.Fatalf("Single = (%v, %v), want ok=false for an activity with no Custom map", v, ok)
	}

	v, ok = def.Single(metrics.Activity{Custom: map[metrics.Key]float64{"other": 5}})
	if ok {
		t.Fatalf("Single = (%v, %v), want ok=false for an activity missing this key", v, ok)
	}
}

// TestNewCustomDef_NonNullableSum confirms a non-nullable custom metric
// (nullable=false) reports ok=true even with zero contributing activities,
// same as every non-nullable built-in sumAcc (e.g. Distance).
func TestNewCustomDef_NonNullableSum(t *testing.T) {
	def := metrics.NewCustomDef("reps", "Reps", "reps", metrics.AggSum, false, false, true, true)
	acc := def.NewAcc()
	acc.Add(metrics.Activity{})

	value, counted, ok := acc.Result()
	if !ok {
		t.Fatal("Result ok = false, want true (non-nullable: an empty sum is a real 0)")
	}
	if counted != 0 || value != 0 {
		t.Fatalf("Result = (%v, %d), want (0, 0)", value, counted)
	}
}
