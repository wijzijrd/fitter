package metrics_test

import (
	"slices"
	"testing"

	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/sports"
)

// Applicability values are plain strings (this package can't import sports at build time), so
// nothing but this test stops a typo like "strenght" from silently hiding a metric from a sport.
func TestApplicabilityOnlyNamesRealSportGroups(t *testing.T) {
	for _, d := range metrics.All() {
		seen := map[string]bool{}
		for _, g := range metrics.ApplicableSportGroups(d.Key) {
			if !sports.ValidGroup(g) {
				t.Errorf("metric %q lists %q, which is not a sport group", d.Key, g)
			}
			if seen[g] {
				t.Errorf("metric %q lists %q twice", d.Key, g)
			}
			seen[g] = true
		}
	}
}

func TestEveryMetricIsMeaningfulForAtLeastOneGroup(t *testing.T) {
	for _, d := range metrics.All() {
		if refs := metrics.ApplicableSportGroups(d.Key); refs != nil && len(refs) == 0 {
			t.Errorf("metric %q has an empty (non-nil) applicability list; nil means every sport", d.Key)
		}
	}
}

// Per-sport view: which metrics a goal on that group may offer.
func TestApplicabilityPerSportGroup(t *testing.T) {
	applies := func(k metrics.Key, g string) bool {
		refs := metrics.ApplicableSportGroups(k)
		return refs == nil || slices.Contains(refs, g)
	}
	distanceShaped := []metrics.Key{metrics.Distance, metrics.ElevationGain, metrics.AverageSpeed, metrics.MaxSpeed, metrics.Pace}
	powerShaped := []metrics.Key{metrics.AverageWatts, metrics.WeightedAverageWatts, metrics.MaxWatts, metrics.Kilojoules, metrics.AverageCadence}

	for _, g := range sports.Groups() {
		key := string(g.Group)
		lifting := g.Group == sports.GroupStrength || g.Group == sports.GroupCircuits
		for _, k := range distanceShaped {
			if got := applies(k, key); got == lifting {
				t.Errorf("%s on %q: applies=%v; distance metrics are for distance sports, not lifting/circuits", k, key, got)
			}
		}
		for _, k := range powerShaped {
			if got, want := applies(k, key), g.Group == sports.GroupRide; got != want {
				t.Errorf("%s on %q: applies=%v, want %v (power metrics are ride-only)", k, key, got, want)
			}
		}
		if got, want := applies(metrics.MaxWeightKg, key), lifting; got != want {
			t.Errorf("max weight on %q: applies=%v, want %v", key, got, want)
		}
		// Time, counts and effort are offered everywhere.
		for _, k := range []metrics.Key{metrics.MovingTime, metrics.ElapsedTime, metrics.ActivityCount, metrics.ActiveDays, metrics.Calories, metrics.SufferScore, metrics.AverageHeartrate, metrics.MaxHeartrate} {
			if !applies(k, key) {
				t.Errorf("%s should apply to %q", k, key)
			}
		}
	}
}
