package progress

import (
	"time"

	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/sports"
)

// Recurrence splits a goal's period into repeating buckets; the goal's
// clauses are evaluated independently inside each one.
type Recurrence int

const (
	RecurNone Recurrence = iota
	RecurDaily
	RecurWeekly  // ISO weeks, Monday start
	RecurMonthly // calendar months
)

// MaxBuckets bounds a recurring goal's bucket count so evaluation stays cheap
// (a daily goal over more than a year is refused at the service boundary).
const MaxBuckets = 400

// bucketStart floors local-clock seconds t to the start of its bucket.
func bucketStart(r Recurrence, t int64) int64 {
	d := time.Unix(t, 0).UTC()
	y, m, day := d.Date()
	switch r {
	case RecurWeekly:
		day -= (int(d.Weekday()) + 6) % 7 // Monday = 0
	case RecurMonthly:
		day = 1
	}
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC).Unix()
}

// bucketEnd is the exclusive end of the bucket starting at start.
func bucketEnd(r Recurrence, start int64) int64 {
	d := time.Unix(start, 0).UTC()
	switch r {
	case RecurWeekly:
		d = d.AddDate(0, 0, 7)
	case RecurMonthly:
		d = d.AddDate(0, 1, 0)
	default:
		d = d.AddDate(0, 0, 1)
	}
	return d.Unix()
}

// CountBuckets is how many buckets a period [start, end] spans.
func CountBuckets(r Recurrence, start, end int64) int {
	n := 0
	for b := bucketStart(r, start); b <= end; b = bucketEnd(r, b) {
		n++
	}
	return n
}

// evaluateRecurring evaluates g.Clauses once per bucket. Result.Ratio is
// buckets hit / buckets elapsed, where the running bucket only counts once
// it is hit (so a fresh week doesn't drag the score down). Result.Satisfied
// means every bucket in the period was hit. Result.Clauses is the running
// bucket's live progress (the last bucket once the period is over).
//
// shortcut: period bounds are treated as local wall-clock instants, so a
// goal's first/last bucket can be off by the user's UTC offset at the edges;
// targets are not pro-rated for partial first/last buckets. Upgrade if users
// create goals that start mid-week.
func evaluateRecurring(g Goal, acts []metrics.Activity) Result {
	byBucket := map[int64][]metrics.Activity{}
	considered := 0
	for _, a := range acts {
		if !sports.Matches(g.Sport, a.SportType, a.Type) || a.StartDate < g.PeriodStart || a.StartDate > g.PeriodEnd {
			continue
		}
		considered++
		b := bucketStart(g.Recurrence, a.LocalStart())
		byBucket[b] = append(byBucket[b], a)
	}

	now := g.Now
	if now == 0 || now > g.PeriodEnd {
		now = g.PeriodEnd
	}

	var starts []int64
	for b := bucketStart(g.Recurrence, g.PeriodStart); b <= g.PeriodEnd; b = bucketEnd(g.Recurrence, b) {
		starts = append(starts, b)
	}
	cur := 0 // index of the running bucket: the last one that has started
	for i, b := range starts {
		if b <= now {
			cur = i
		}
	}

	hit := 0
	for _, b := range starts[:cur] {
		if evaluateFiltered(g, byBucket[b]).Satisfied {
			hit++
		}
	}
	running := evaluateFiltered(g, byBucket[starts[cur]])
	elapsed := cur
	if running.Satisfied {
		hit++
		elapsed++
	}

	running.GoalID = g.ID
	running.ConsideredCount = considered
	running.Ratio = 0
	if elapsed > 0 {
		running.Ratio = float64(hit) / float64(elapsed)
	}
	running.Satisfied = hit == len(starts)
	running.PeriodsHit, running.PeriodsElapsed, running.PeriodsTotal = hit, elapsed, len(starts)
	running.QualifyingActivityID, running.BestAttemptActivityID, running.SatisfiedAt = 0, 0, 0
	return running
}
