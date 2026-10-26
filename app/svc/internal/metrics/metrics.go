// Package metrics defines the metric taxonomy used by goal clauses:
// definitions, units, and aggregation semantics. Everything else
// (validation, aggregation, the proto enum, UI dropdowns, formatting)
// derives from the table in this package.
//
// This package deliberately has no dependency on the database schema: it
// works over its own narrow Activity value type, not db.Activity, so it is
// fully unit-testable with hand-built literals. A one-file adapter
// converting db.Activity to metrics.Activity is added in a later task, once
// the schema exists.
package metrics

// Key identifies one metric in the taxonomy.
type Key string

const (
	Distance             Key = "distance"
	ElevationGain        Key = "elevation_gain"
	MovingTime           Key = "moving_time"
	ElapsedTime          Key = "elapsed_time"
	ActivityCount        Key = "activity_count"
	AverageSpeed         Key = "average_speed"
	MaxSpeed             Key = "max_speed"
	Pace                 Key = "pace"
	AverageHeartrate     Key = "average_heartrate"
	MaxHeartrate         Key = "max_heartrate"
	AverageWatts         Key = "average_watts"
	WeightedAverageWatts Key = "weighted_average_watts"
	MaxWatts             Key = "max_watts"
	Kilojoules           Key = "kilojoules"
	Calories             Key = "calories"
	AverageCadence       Key = "average_cadence"
	SufferScore          Key = "suffer_score"
)

// Activity is the engine's own narrow view of an activity — deliberately
// not db.Activity, so this package has no dependency on the schema and is
// fully testable with literals.
//
// Nullable fields are *float64 so "absent" is distinguishable from "a real
// zero" (e.g. average_heartrate: 0 from a disconnected strap must not read
// as a 0 bpm reading).
type Activity struct {
	StravaActivityID int64
	UserID           string
	Type             string
	SportType        string
	StartDate        int64 // unix seconds
	DistanceM        float64
	ElevationM       float64
	MovingTimeS      float64
	ElapsedTimeS     float64
	AverageSpeedMS   float64
	MaxSpeedMS       float64
	AverageHeartrate *float64
	MaxHeartrate     *float64
	AverageWatts     *float64
	WeightedAvgWatts *float64
	MaxWatts         *float64
	Kilojoules       *float64
	Calories         *float64
	AverageCadence   *float64
	SufferScore      *float64
	// Custom carries admin-defined custom-metric values (see NewCustomDef),
	// keyed by the same Key type built-ins use. A nil map, or a key simply
	// absent from it, means "not present" — matching the *float64 nullable
	// fields' semantics for built-ins, just via map membership instead of a
	// pointer. FromDB never sets this (see adapter.go); the service layer
	// populates it one layer up, from real DB columns for built-in metrics
	// logged manually and from manual_entries/synthetic activities for
	// custom ones (Task 5).
	Custom map[Key]float64
}

// Aggregation identifies how an Accumulator combines activities into one
// value.
type Aggregation int

const (
	AggSum Aggregation = iota + 1
	AggWeightedAvg
	AggMax
	AggCount
	AggDerived
)

// Unit identifies how a metric's numeric value should be interpreted and
// formatted.
type Unit int

const (
	UnitMeters Unit = iota + 1
	UnitSeconds
	UnitMetersPerSecond
	UnitSecondsPerKm
	UnitBPM
	UnitWatts
	UnitKilojoules
	UnitKilocalories
	UnitRPM
	UnitCount
	UnitScore
)

// Accumulator folds activities into one aggregate value. Result() is
// callable after every Add(), which is what lets the timeline (a later
// task) produce a running aggregate over time using the exact same code as
// the goal-period aggregate.
type Accumulator interface {
	Add(a Activity)
	// Result returns the aggregate value, how many activities actually
	// supplied it, and whether the result is meaningful at all. ok is
	// false when: (a) a nullable metric had zero contributing activities,
	// (b) a derived metric like pace has no distance to divide by, or (c)
	// an AVERAGE or MAX aggregation had zero contributing activities at
	// all — this last case holds regardless of nullability, since an
	// average or max of nothing is undefined, unlike SUM/COUNT where zero
	// contributing activities is a real, meaningful value of 0.
	Result() (value float64, counted int, ok bool)
}

// Def describes one metric in the taxonomy: its unit, aggregation
// semantics, and how to read it from a single Activity or fold it over
// many.
type Def struct {
	Key               Key
	Label             string
	Unit              Unit
	Aggregation       Aggregation
	Nullable          bool
	LowerIsBetter     bool // display hint only — must never affect evaluation logic
	SupportsSingle    bool // false only for ActivityCount (a per-activity count is meaningless)
	SupportsAggregate bool // true for every metric
	// Single returns the per-activity value, for SINGLE-scope clauses.
	Single func(a Activity) (value float64, ok bool)
	// NewAcc returns a fresh Accumulator for AGGREGATE-scope clauses and
	// timeline computation.
	NewAcc func() Accumulator
}

// extractor reads one field off an Activity, reporting whether it was
// present. Non-nullable fields are always present; nullable (*float64)
// fields report absence via ok=false. The same extractor backs both a
// metric's Single func and its Accumulator, so the two can never disagree
// about what counts as "present".
type extractor func(a Activity) (value float64, ok bool)

func nonNullable(get func(a Activity) float64) extractor {
	return func(a Activity) (float64, bool) { return get(a), true }
}

func nullable(get func(a Activity) *float64) extractor {
	return func(a Activity) (float64, bool) {
		p := get(a)
		if p == nil {
			return 0, false
		}
		return *p, true
	}
}

// defs is the single source of truth for the metric taxonomy. Order is
// user-visible (metadata RPC, UI dropdown) and must stay stable.
var defs = buildDefs()

func buildDefs() []Def {
	distance := nonNullable(func(a Activity) float64 { return a.DistanceM })
	elevation := nonNullable(func(a Activity) float64 { return a.ElevationM })
	movingTime := nonNullable(func(a Activity) float64 { return a.MovingTimeS })
	elapsedTime := nonNullable(func(a Activity) float64 { return a.ElapsedTimeS })
	averageSpeed := nonNullable(func(a Activity) float64 { return a.AverageSpeedMS })
	maxSpeed := nonNullable(func(a Activity) float64 { return a.MaxSpeedMS })
	averageHeartrate := nullable(func(a Activity) *float64 { return a.AverageHeartrate })
	maxHeartrate := nullable(func(a Activity) *float64 { return a.MaxHeartrate })
	averageWatts := nullable(func(a Activity) *float64 { return a.AverageWatts })
	weightedAvgWatts := nullable(func(a Activity) *float64 { return a.WeightedAvgWatts })
	maxWatts := nullable(func(a Activity) *float64 { return a.MaxWatts })
	kilojoules := nullable(func(a Activity) *float64 { return a.Kilojoules })
	calories := nullable(func(a Activity) *float64 { return a.Calories })
	averageCadence := nullable(func(a Activity) *float64 { return a.AverageCadence })
	sufferScore := nullable(func(a Activity) *float64 { return a.SufferScore })
	pace := extractor(func(a Activity) (float64, bool) {
		if a.DistanceM <= 0 {
			return 0, false
		}
		return a.MovingTimeS / (a.DistanceM / 1000), true
	})

	return []Def{
		{
			Key: Distance, Label: "Distance", Unit: UnitMeters, Aggregation: AggSum,
			SupportsSingle: true, SupportsAggregate: true,
			Single: distance, NewAcc: func() Accumulator { return &sumAcc{extract: distance} },
		},
		{
			Key: ElevationGain, Label: "Elevation gain", Unit: UnitMeters, Aggregation: AggSum,
			SupportsSingle: true, SupportsAggregate: true,
			Single: elevation, NewAcc: func() Accumulator { return &sumAcc{extract: elevation} },
		},
		{
			Key: MovingTime, Label: "Moving time", Unit: UnitSeconds, Aggregation: AggSum,
			SupportsSingle: true, SupportsAggregate: true,
			Single: movingTime, NewAcc: func() Accumulator { return &sumAcc{extract: movingTime} },
		},
		{
			Key: ElapsedTime, Label: "Elapsed time", Unit: UnitSeconds, Aggregation: AggSum,
			SupportsSingle: true, SupportsAggregate: true,
			Single: elapsedTime, NewAcc: func() Accumulator { return &sumAcc{extract: elapsedTime} },
		},
		{
			// A per-activity count is meaningless; SupportsSingle=false
			// rejects SINGLE-scope clauses at the service boundary
			// (Task 7). Single still returns a well-defined value rather
			// than erroring, since nothing here should ever be asked to
			// silently accept a SINGLE clause on this key.
			Key: ActivityCount, Label: "Activity count", Unit: UnitCount, Aggregation: AggCount,
			SupportsSingle: false, SupportsAggregate: true,
			Single: func(a Activity) (float64, bool) { return 1, true },
			NewAcc: func() Accumulator { return &countAcc{} },
		},
		{
			Key: AverageSpeed, Label: "Average speed", Unit: UnitMetersPerSecond, Aggregation: AggWeightedAvg,
			SupportsSingle: true, SupportsAggregate: true,
			Single: averageSpeed, NewAcc: func() Accumulator { return &weightedAvgAcc{extract: averageSpeed} },
		},
		{
			Key: MaxSpeed, Label: "Max speed", Unit: UnitMetersPerSecond, Aggregation: AggMax,
			SupportsSingle: true, SupportsAggregate: true,
			Single: maxSpeed, NewAcc: func() Accumulator { return &maxAcc{extract: maxSpeed} },
		},
		{
			Key: Pace, Label: "Pace", Unit: UnitSecondsPerKm, Aggregation: AggDerived, LowerIsBetter: true,
			SupportsSingle: true, SupportsAggregate: true,
			Single: pace, NewAcc: func() Accumulator { return &paceAcc{} },
		},
		{
			Key: AverageHeartrate, Label: "Average heart rate", Unit: UnitBPM, Aggregation: AggWeightedAvg, Nullable: true,
			SupportsSingle: true, SupportsAggregate: true,
			Single: averageHeartrate, NewAcc: func() Accumulator { return &weightedAvgAcc{extract: averageHeartrate} },
		},
		{
			Key: MaxHeartrate, Label: "Max heart rate", Unit: UnitBPM, Aggregation: AggMax, Nullable: true,
			SupportsSingle: true, SupportsAggregate: true,
			Single: maxHeartrate, NewAcc: func() Accumulator { return &maxAcc{extract: maxHeartrate} },
		},
		{
			Key: AverageWatts, Label: "Average power", Unit: UnitWatts, Aggregation: AggWeightedAvg, Nullable: true,
			SupportsSingle: true, SupportsAggregate: true,
			Single: averageWatts, NewAcc: func() Accumulator { return &weightedAvgAcc{extract: averageWatts} },
		},
		{
			Key: WeightedAverageWatts, Label: "Weighted average power", Unit: UnitWatts, Aggregation: AggWeightedAvg, Nullable: true,
			SupportsSingle: true, SupportsAggregate: true,
			Single: weightedAvgWatts, NewAcc: func() Accumulator { return &weightedAvgAcc{extract: weightedAvgWatts} },
		},
		{
			Key: MaxWatts, Label: "Max power", Unit: UnitWatts, Aggregation: AggMax, Nullable: true,
			SupportsSingle: true, SupportsAggregate: true,
			Single: maxWatts, NewAcc: func() Accumulator { return &maxAcc{extract: maxWatts} },
		},
		{
			Key: Kilojoules, Label: "Energy", Unit: UnitKilojoules, Aggregation: AggSum, Nullable: true,
			SupportsSingle: true, SupportsAggregate: true,
			Single: kilojoules, NewAcc: func() Accumulator { return &sumAcc{nullable: true, extract: kilojoules} },
		},
		{
			Key: Calories, Label: "Calories", Unit: UnitKilocalories, Aggregation: AggSum, Nullable: true,
			SupportsSingle: true, SupportsAggregate: true,
			Single: calories, NewAcc: func() Accumulator { return &sumAcc{nullable: true, extract: calories} },
		},
		{
			Key: AverageCadence, Label: "Average cadence", Unit: UnitRPM, Aggregation: AggWeightedAvg, Nullable: true,
			SupportsSingle: true, SupportsAggregate: true,
			Single: averageCadence, NewAcc: func() Accumulator { return &weightedAvgAcc{extract: averageCadence} },
		},
		{
			Key: SufferScore, Label: "Relative effort", Unit: UnitScore, Aggregation: AggSum, Nullable: true,
			SupportsSingle: true, SupportsAggregate: true,
			Single: sufferScore, NewAcc: func() Accumulator { return &sumAcc{nullable: true, extract: sufferScore} },
		},
	}
}

var defsByKey = func() map[Key]Def {
	m := make(map[Key]Def, len(defs))
	for _, d := range defs {
		m[d.Key] = d
	}
	return m
}()

// Lookup returns the Def for k, and whether k is a known metric.
func Lookup(k Key) (Def, bool) {
	d, ok := defsByKey[k]
	return d, ok
}

// All returns every metric definition in stable order, which drives the
// metadata RPC and its test.
func All() []Def {
	out := make([]Def, len(defs))
	copy(out, defs)
	return out
}
