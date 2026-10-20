package metrics_test

import (
	"math"
	"testing"

	"github.com/fitter/svc/internal/metrics"
)

func ptr(f float64) *float64 { return &f }

func TestAllDefsHaveSingleAndNewAcc(t *testing.T) {
	all := metrics.All()
	if len(all) != 17 {
		t.Fatalf("All() returned %d defs, want 17", len(all))
	}
	seen := make(map[metrics.Key]bool)
	for _, d := range all {
		if d.Single == nil {
			t.Errorf("%s: Single is nil", d.Key)
		}
		if d.NewAcc == nil {
			t.Errorf("%s: NewAcc is nil", d.Key)
		}
		if d.NewAcc != nil && d.NewAcc() == nil {
			t.Errorf("%s: NewAcc() returned nil Accumulator", d.Key)
		}
		if !d.SupportsAggregate {
			t.Errorf("%s: SupportsAggregate must be true for every metric", d.Key)
		}
		if seen[d.Key] {
			t.Errorf("%s: duplicate key in All()", d.Key)
		}
		seen[d.Key] = true
	}
}

func TestAllStableOrder(t *testing.T) {
	first := metrics.All()
	second := metrics.All()
	if len(first) != len(second) {
		t.Fatalf("All() length changed between calls: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Key != second[i].Key {
			t.Fatalf("All() order changed at index %d: %s vs %s", i, first[i].Key, second[i].Key)
		}
	}
}

func TestOnlyActivityCountRejectsSingle(t *testing.T) {
	for _, d := range metrics.All() {
		if d.Key == metrics.ActivityCount {
			if d.SupportsSingle {
				t.Errorf("%s: SupportsSingle should be false", d.Key)
			}
			continue
		}
		if !d.SupportsSingle {
			t.Errorf("%s: SupportsSingle should be true", d.Key)
		}
	}
}

func TestLookup(t *testing.T) {
	d, ok := metrics.Lookup(metrics.Distance)
	if !ok {
		t.Fatal("Lookup(Distance) not found")
	}
	if d.Key != metrics.Distance {
		t.Fatalf("Lookup(Distance).Key = %s, want %s", d.Key, metrics.Distance)
	}
	if _, ok := metrics.Lookup(metrics.Key("nonexistent")); ok {
		t.Fatal("Lookup(nonexistent) should not be found")
	}
}

// TestNullableSingleZeroValue verifies every nullable Def's Single returns
// ok=false on a zero-value Activity, and ok=true once the corresponding
// pointer field is set — the core "no data must never read as a real
// value" invariant.
func TestNullableSingleZeroValue(t *testing.T) {
	cases := []struct {
		key metrics.Key
		set func(a *metrics.Activity)
	}{
		{metrics.AverageHeartrate, func(a *metrics.Activity) { a.AverageHeartrate = ptr(0) }},
		{metrics.MaxHeartrate, func(a *metrics.Activity) { a.MaxHeartrate = ptr(0) }},
		{metrics.AverageWatts, func(a *metrics.Activity) { a.AverageWatts = ptr(0) }},
		{metrics.WeightedAverageWatts, func(a *metrics.Activity) { a.WeightedAvgWatts = ptr(0) }},
		{metrics.MaxWatts, func(a *metrics.Activity) { a.MaxWatts = ptr(0) }},
		{metrics.Kilojoules, func(a *metrics.Activity) { a.Kilojoules = ptr(0) }},
		{metrics.Calories, func(a *metrics.Activity) { a.Calories = ptr(0) }},
		{metrics.AverageCadence, func(a *metrics.Activity) { a.AverageCadence = ptr(0) }},
		{metrics.SufferScore, func(a *metrics.Activity) { a.SufferScore = ptr(0) }},
	}

	for _, tc := range cases {
		d, ok := metrics.Lookup(tc.key)
		if !ok {
			t.Fatalf("%s: not found", tc.key)
		}
		if !d.Nullable {
			t.Errorf("%s: expected Nullable=true", tc.key)
		}

		if _, ok := d.Single(metrics.Activity{}); ok {
			t.Errorf("%s: Single(zero value) should return ok=false", tc.key)
		}

		var a metrics.Activity
		tc.set(&a)
		v, ok := d.Single(a)
		if !ok {
			t.Errorf("%s: Single(with pointer set to 0) should return ok=true", tc.key)
		}
		if v != 0 {
			t.Errorf("%s: Single value = %v, want 0 (a real zero, not absence)", tc.key, v)
		}
	}
}

func TestNonNullableDefsAreNotNullable(t *testing.T) {
	nullableKeys := map[metrics.Key]bool{
		metrics.AverageHeartrate:     true,
		metrics.MaxHeartrate:         true,
		metrics.AverageWatts:         true,
		metrics.WeightedAverageWatts: true,
		metrics.MaxWatts:             true,
		metrics.Kilojoules:           true,
		metrics.Calories:             true,
		metrics.AverageCadence:       true,
		metrics.SufferScore:          true,
	}
	for _, d := range metrics.All() {
		want := nullableKeys[d.Key]
		if d.Nullable != want {
			t.Errorf("%s: Nullable = %v, want %v", d.Key, d.Nullable, want)
		}
	}
}

func TestSumAccBasic(t *testing.T) {
	d, _ := metrics.Lookup(metrics.Distance)
	acc := d.NewAcc()
	acc.Add(metrics.Activity{DistanceM: 1000})
	acc.Add(metrics.Activity{DistanceM: 2500})
	v, counted, ok := acc.Result()
	if !ok || counted != 2 || v != 3500 {
		t.Fatalf("sumAcc(distance) = (%v, %d, %v), want (3500, 2, true)", v, counted, ok)
	}
}

func TestSumAccEmptyNonNullableIsOK(t *testing.T) {
	d, _ := metrics.Lookup(metrics.Distance)
	acc := d.NewAcc()
	v, counted, ok := acc.Result()
	if !ok || counted != 0 || v != 0 {
		t.Fatalf("sumAcc(distance) on empty set = (%v, %d, %v), want (0, 0, true)", v, counted, ok)
	}
}

func TestSumAccNullableSkipsAbsentAndEmptySetIsNotOK(t *testing.T) {
	d, _ := metrics.Lookup(metrics.Calories)
	acc := d.NewAcc()
	// No activities at all.
	if _, _, ok := acc.Result(); ok {
		t.Fatal("sumAcc(calories) on empty set should be ok=false (nullable, no contributors)")
	}

	acc = d.NewAcc()
	acc.Add(metrics.Activity{})                   // no Calories set: skipped
	acc.Add(metrics.Activity{Calories: ptr(100)}) // counted
	acc.Add(metrics.Activity{Calories: ptr(50)})  // counted
	v, counted, ok := acc.Result()
	if !ok || counted != 2 || v != 150 {
		t.Fatalf("sumAcc(calories) = (%v, %d, %v), want (150, 2, true)", v, counted, ok)
	}
}

func TestCountAccCountsRegardlessOfFields(t *testing.T) {
	d, _ := metrics.Lookup(metrics.ActivityCount)
	acc := d.NewAcc()
	// Empty set: legitimately satisfies e.g. "<= 10", so ok=true, value=0.
	v, counted, ok := acc.Result()
	if !ok || counted != 0 || v != 0 {
		t.Fatalf("countAcc empty = (%v, %d, %v), want (0, 0, true)", v, counted, ok)
	}

	acc.Add(metrics.Activity{})
	acc.Add(metrics.Activity{DistanceM: 5000})
	acc.Add(metrics.Activity{})
	v, counted, ok = acc.Result()
	if !ok || counted != 3 || v != 3 {
		t.Fatalf("countAcc after 3 adds = (%v, %d, %v), want (3, 3, true)", v, counted, ok)
	}
}

func TestWeightedAvgAccDurationWeighted(t *testing.T) {
	d, _ := metrics.Lookup(metrics.AverageHeartrate)
	acc := d.NewAcc()
	// A 4-hour ride at 150 bpm and a 20-minute jog at 170 bpm must not
	// average to 160 — the 4-hour ride should dominate.
	acc.Add(metrics.Activity{MovingTimeS: 4 * 3600, AverageHeartrate: ptr(150)})
	acc.Add(metrics.Activity{MovingTimeS: 20 * 60, AverageHeartrate: ptr(170)})
	v, counted, ok := acc.Result()
	if !ok || counted != 2 {
		t.Fatalf("weightedAvgAcc = (%v, %d, %v), want ok=true, counted=2", v, counted, ok)
	}
	wantWeightedSum := 150.0*4*3600 + 170.0*20*60
	wantWeight := 4*3600.0 + 20*60.0
	want := wantWeightedSum / wantWeight
	if math.Abs(v-want) > 1e-9 {
		t.Fatalf("weightedAvgAcc value = %v, want %v (naive mean would be 160)", v, want)
	}
	if math.Abs(v-160) < 1 {
		t.Fatalf("weightedAvgAcc value = %v looks like a naive mean (160), not duration-weighted", v)
	}
}

func TestWeightedAvgAccSkipsZeroMovingTimeAndAbsentValues(t *testing.T) {
	d, _ := metrics.Lookup(metrics.AverageWatts)
	acc := d.NewAcc()
	acc.Add(metrics.Activity{MovingTimeS: 0, AverageWatts: ptr(999)}) // zero weight: skipped
	acc.Add(metrics.Activity{MovingTimeS: 100})                       // no watts: skipped
	acc.Add(metrics.Activity{MovingTimeS: 100, AverageWatts: ptr(200)})
	v, counted, ok := acc.Result()
	if !ok || counted != 1 || v != 200 {
		t.Fatalf("weightedAvgAcc = (%v, %d, %v), want (200, 1, true)", v, counted, ok)
	}
}

func TestWeightedAvgAccEmptySetNotOK(t *testing.T) {
	d, _ := metrics.Lookup(metrics.AverageSpeed)
	acc := d.NewAcc()
	if _, _, ok := acc.Result(); ok {
		t.Fatal("weightedAvgAcc on empty set should be ok=false")
	}
}

func TestMaxAccRunningMax(t *testing.T) {
	d, _ := metrics.Lookup(metrics.MaxSpeed)
	acc := d.NewAcc()
	acc.Add(metrics.Activity{MaxSpeedMS: 5})
	acc.Add(metrics.Activity{MaxSpeedMS: 12})
	acc.Add(metrics.Activity{MaxSpeedMS: 8})
	v, counted, ok := acc.Result()
	if !ok || counted != 3 || v != 12 {
		t.Fatalf("maxAcc = (%v, %d, %v), want (12, 3, true)", v, counted, ok)
	}
}

func TestMaxAccNullableSkipsAbsent(t *testing.T) {
	d, _ := metrics.Lookup(metrics.MaxHeartrate)
	acc := d.NewAcc()
	acc.Add(metrics.Activity{}) // absent: skipped
	acc.Add(metrics.Activity{MaxHeartrate: ptr(140)})
	v, counted, ok := acc.Result()
	if !ok || counted != 1 || v != 140 {
		t.Fatalf("maxAcc(heartrate) = (%v, %d, %v), want (140, 1, true)", v, counted, ok)
	}
}

func TestMaxAccEmptySetNotOK(t *testing.T) {
	d, _ := metrics.Lookup(metrics.MaxSpeed)
	acc := d.NewAcc()
	if _, _, ok := acc.Result(); ok {
		t.Fatal("maxAcc on empty set should be ok=false")
	}
}

func TestPaceAccDerivedFromSums(t *testing.T) {
	d, _ := metrics.Lookup(metrics.Pace)
	acc := d.NewAcc()
	acc.Add(metrics.Activity{DistanceM: 5000, MovingTimeS: 1500}) // 5km in 25min
	acc.Add(metrics.Activity{DistanceM: 5000, MovingTimeS: 1800}) // 5km in 30min
	v, counted, ok := acc.Result()
	if !ok || counted != 2 {
		t.Fatalf("paceAcc = (%v, %d, %v), want ok=true, counted=2", v, counted, ok)
	}
	// Total: 10km in 3300s -> 330 s/km, not the mean of (300, 360) = 330
	// coincidentally equal here; use an uneven split to actually
	// distinguish sum-of-sums from mean-of-paces below.
	want := 3300.0 / 10.0
	if math.Abs(v-want) > 1e-9 {
		t.Fatalf("paceAcc value = %v, want %v", v, want)
	}
}

func TestPaceAccIsSumRatioNotMeanOfPaces(t *testing.T) {
	d, _ := metrics.Lookup(metrics.Pace)
	acc := d.NewAcc()
	// 1km in 300s (pace 300 s/km) and 9km in 1800s (pace 200 s/km).
	// Mean of per-activity paces = 250 s/km. Correct sum-ratio pace =
	// (300+1800)/(1+9) = 210 s/km.
	acc.Add(metrics.Activity{DistanceM: 1000, MovingTimeS: 300})
	acc.Add(metrics.Activity{DistanceM: 9000, MovingTimeS: 1800})
	v, _, ok := acc.Result()
	if !ok {
		t.Fatal("paceAcc should be ok=true")
	}
	if math.Abs(v-210) > 1e-9 {
		t.Fatalf("paceAcc value = %v, want 210 (sum-ratio, not mean-of-paces 250)", v)
	}
}

func TestPaceAccZeroDistanceNotOK(t *testing.T) {
	d, _ := metrics.Lookup(metrics.Pace)
	acc := d.NewAcc()
	acc.Add(metrics.Activity{DistanceM: 0, MovingTimeS: 600})
	if _, _, ok := acc.Result(); ok {
		t.Fatal("paceAcc with zero total distance should be ok=false")
	}
}

func TestPaceSingleZeroDistanceNotOK(t *testing.T) {
	d, _ := metrics.Lookup(metrics.Pace)
	if _, ok := d.Single(metrics.Activity{DistanceM: 0, MovingTimeS: 600}); ok {
		t.Fatal("Pace.Single with zero distance should be ok=false")
	}
}

func TestActivityCountSingleIgnoresSupportsSingleFlag(t *testing.T) {
	// ActivityCount.Single still returns a defined value even though
	// SupportsSingle is false — rejecting a SINGLE-scope activity_count
	// clause is the service boundary's job (Task 7), not this package's.
	d, _ := metrics.Lookup(metrics.ActivityCount)
	v, ok := d.Single(metrics.Activity{})
	if !ok || v != 1 {
		t.Fatalf("ActivityCount.Single = (%v, %v), want (1, true)", v, ok)
	}
}
