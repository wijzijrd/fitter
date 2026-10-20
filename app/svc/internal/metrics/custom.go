package metrics

// NewCustomDef builds a Def for an admin-defined custom metric (see
// custom_metrics in the DB schema), backed by Activity.Custom instead of a
// named struct field. It is the ONLY way this package's Def type is ever
// constructed outside buildDefs — every built-in Def stays exactly as it
// was, byte-for-byte, before this function existed.
//
// The Def it returns satisfies the identical contract every built-in Def
// already does: Single returns (value, ok) exactly like every other
// extractor-backed metric, and NewAcc's Accumulator honors the same
// Result() (value, counted, ok) three-value shape documented on the
// Accumulator interface. A caller (progress.Evaluate, the timeline, the
// leaderboard) can't tell a custom Def apart from a built-in one by
// behavior — only by Key.
//
// unit is a free-text display unit (e.g. "kg", "reps") — custom metrics
// aren't constrained to the closed built-in Unit enum, so the returned
// Def's Unit field is left at its zero value (unspecified). The caller's
// own custom-metric record (not Def) is the source of truth for the raw
// unit string; nothing in this package's evaluation path ever reads
// Def.Unit, so this loses no behavior.
//
// aggregation must be one of AggSum, AggWeightedAvg, AggMax, or AggCount —
// custom_metrics.aggregation is DB-CHECK-constrained to sum/average/max/
// count (never "derived", which only Pace uses, since a derived aggregation
// needs bespoke per-metric math no generic factory can express). An
// unrecognized aggregation value here is defensive-only and falls back to
// AggSum's accumulator rather than returning a Def with a nil NewAcc, which
// would panic the first caller.
func NewCustomDef(key Key, label string, unit string, aggregation Aggregation, nullable, lowerIsBetter, supportsSingle, supportsAggregate bool) Def {
	extract := extractor(func(a Activity) (float64, bool) {
		v, ok := a.Custom[key]
		return v, ok
	})

	var newAcc func() Accumulator
	switch aggregation {
	case AggSum:
		newAcc = func() Accumulator { return &sumAcc{nullable: nullable, extract: extract} }
	case AggWeightedAvg:
		newAcc = func() Accumulator { return &weightedAvgAcc{extract: extract} }
	case AggMax:
		newAcc = func() Accumulator { return &maxAcc{extract: extract} }
	case AggCount:
		newAcc = func() Accumulator { return &presenceCountAcc{extract: extract} }
	default:
		newAcc = func() Accumulator { return &sumAcc{nullable: nullable, extract: extract} }
	}

	return Def{
		Key:               key,
		Label:             label,
		Unit:              0,
		Aggregation:       aggregation,
		Nullable:          nullable,
		LowerIsBetter:     lowerIsBetter,
		SupportsSingle:    supportsSingle,
		SupportsAggregate: supportsAggregate,
		Single:            extract,
		NewAcc:            newAcc,
	}
}

// presenceCountAcc counts activities where the custom metric was actually
// present (extract returns ok=true), unlike the built-in countAcc (used
// only for ActivityCount) which counts every activity unconditionally.
// This distinction matters here: a custom AggCount metric folds over the
// same filtered activity set every other clause on the goal does, most of
// which will never carry this particular custom key at all — counting them
// anyway would silently overcount. Zero contributing activities is still a
// real, meaningful 0 (not "unknown"), so ok is always true — same reasoning
// as countAcc.
type presenceCountAcc struct {
	extract extractor
	counted int
}

func (a *presenceCountAcc) Add(act Activity) {
	if _, ok := a.extract(act); ok {
		a.counted++
	}
}

func (a *presenceCountAcc) Result() (float64, int, bool) {
	return float64(a.counted), a.counted, true
}
