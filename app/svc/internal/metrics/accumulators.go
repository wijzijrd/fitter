package metrics

// sumAcc sums a field across activities: distance, elevation_gain,
// moving_time, elapsed_time, kilojoules, calories, suffer_score.
//
// Activities where the (possibly nullable) value is absent are skipped;
// counted tracks how many activities actually contributed. For a
// non-nullable metric, an empty (or all-absent, which can't happen for a
// non-nullable field) set legitimately sums to 0, so ok is always true. For
// a nullable metric, ok is false when nothing contributed — "no data" must
// never read as "target met".
type sumAcc struct {
	nullable bool
	extract  extractor
	sum      float64
	counted  int
}

func (a *sumAcc) Add(act Activity) {
	v, ok := a.extract(act)
	if !ok {
		return
	}
	a.sum += v
	a.counted++
}

func (a *sumAcc) Result() (float64, int, bool) {
	ok := a.counted > 0 || !a.nullable
	return a.sum, a.counted, ok
}

// countAcc counts every activity passed to Add, regardless of any field
// value. Never nullable: an empty set legitimately satisfies e.g.
// "activity_count <= 10", so ok is always true.
type countAcc struct {
	counted int
}

func (a *countAcc) Add(Activity) { a.counted++ }

func (a *countAcc) Result() (float64, int, bool) {
	return float64(a.counted), a.counted, true
}

// weightedAvgAcc computes a duration-weighted average: average_speed,
// average_heartrate, average_watts, weighted_average_watts,
// average_cadence. It accumulates Σ(value × moving_time_s) and
// Σ(moving_time_s) over activities that have the value, dividing at
// Result(). Activities with moving_time_s == 0 are skipped entirely — zero
// weight, and including them risks a divide-by-zero if they're the only
// rows. An empty contributing set (no data, or nothing with positive
// moving time) makes the average undefined, so ok is false regardless of
// whether the metric is nullable.
type weightedAvgAcc struct {
	extract     extractor
	weightedSum float64
	weightSum   float64
	counted     int
}

func (a *weightedAvgAcc) Add(act Activity) {
	if act.MovingTimeS == 0 {
		return
	}
	v, ok := a.extract(act)
	if !ok {
		return
	}
	a.weightedSum += v * act.MovingTimeS
	a.weightSum += act.MovingTimeS
	a.counted++
}

func (a *weightedAvgAcc) Result() (float64, int, bool) {
	if a.weightSum == 0 {
		return 0, a.counted, false
	}
	return a.weightedSum / a.weightSum, a.counted, true
}

// maxAcc tracks a running max over present values only: max_speed,
// max_heartrate, max_watts. An empty contributing set makes the max
// undefined, so ok is false regardless of whether the metric is nullable.
type maxAcc struct {
	extract extractor
	max     float64
	counted int
}

func (a *maxAcc) Add(act Activity) {
	v, ok := a.extract(act)
	if !ok {
		return
	}
	if a.counted == 0 || v > a.max {
		a.max = v
	}
	a.counted++
}

func (a *maxAcc) Result() (float64, int, bool) {
	return a.max, a.counted, a.counted > 0
}

// paceAcc accumulates Σmoving_time_s and Σdistance_m separately (never as
// a mean of per-activity paces — see the package-level duration-weighting
// note). Result returns seconds-per-km; ok is false when summed distance
// is 0, since pace is undefined without distance.
type paceAcc struct {
	movingTimeS float64
	distanceM   float64
	counted     int
}

func (a *paceAcc) Add(act Activity) {
	a.movingTimeS += act.MovingTimeS
	a.distanceM += act.DistanceM
	a.counted++
}

func (a *paceAcc) Result() (float64, int, bool) {
	if a.distanceM <= 0 {
		return 0, a.counted, false
	}
	return a.movingTimeS / (a.distanceM / 1000), a.counted, true
}
