package metrics

// builtinApplicability is curated "which sport groups is this metric
// meaningful for" tag data for the 17 built-in metrics, exactly like
// internal/sports's own group map: hand-maintained curation, not derived
// from anything else, versioned in code.
//
// Values are plain strings matching internal/sports.Group's string values
// (e.g. "strength", "ride") rather than the sports.Group type itself — this
// package deliberately has no dependency on internal/sports (see the
// package doc in metrics.go), and the wire representation
// (MetricInfo.applicable_sport_groups) is `repeated string` anyway, so a
// plain string here avoids a needless type round-trip at the one call site
// that ever reads this map.
//
// A key absent from this map (or mapped to a nil/empty slice) means
// "applies to every sport" — matching today's implicit behavior for every
// metric before this map existed.
var builtinApplicability = map[Key][]string{
	// Distance-shaped metrics: meaningful for anything with a distance
	// component, meaningless for strength training (no distance is
	// recorded, and "how far did you lift" isn't a real quantity).
	Distance:      {"run", "ride", "swim", "walk", "hike", "winter", "water", "other"},
	ElevationGain: {"run", "ride", "swim", "walk", "hike", "winter", "water", "other"},
	AverageSpeed:  {"run", "ride", "swim", "walk", "hike", "winter", "water", "other"},
	MaxSpeed:      {"run", "ride", "swim", "walk", "hike", "winter", "water", "other"},
	Pace:          {"run", "ride", "swim", "walk", "hike", "winter", "water", "other"},

	// Duration/count/effort metrics apply broadly, strength included —
	// every sport has a moving/elapsed time, an activity count, and can
	// report relative effort or heart rate.
	MovingTime:       nil,
	ElapsedTime:      nil,
	ActivityCount:    nil,
	AverageHeartrate: nil,
	MaxHeartrate:     nil,
	Calories:         nil,
	SufferScore:      nil,

	// Power-meter-derived metrics: primarily a cycling concept in this
	// taxonomy today (no run-power support), so scoped to ride only.
	AverageWatts:         {"ride"},
	WeightedAverageWatts: {"ride"},
	MaxWatts:             {"ride"},
	Kilojoules:           {"ride"}, // always paired with a power meter reading, same family as watts
	AverageCadence:       {"ride"}, // Strava also reports running cadence, but this taxonomy curates it as cycling-primary
}

// ApplicableSportGroups returns the curated sport groups k is meaningful
// for, or nil if k applies to every sport (no curated restriction). Callers
// must not mutate the returned slice.
func ApplicableSportGroups(k Key) []string {
	return builtinApplicability[k]
}
