// Package progress implements the clause-based goal evaluator: given a Goal
// (a sport scope, a period, and a list of clauses) and a caller-supplied set
// of activities, it reports whether the goal is satisfied and how close it
// is.
//
// This package deliberately has no dependency on the database schema: it
// works over metrics.Activity, the same narrow, DB-free value type used by
// internal/metrics, so it is fully unit-testable with hand-built literals.
// Evaluate itself does the sport+period filtering, which is what makes it
// pure and independent of how the caller fetched the activities.
package progress

import (
	"sort"

	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/sports"
)

// Scope identifies whether a clause is evaluated against an aggregate over
// every considered activity, or against a single activity in isolation.
type Scope int

const (
	ScopeAggregate Scope = iota + 1
	ScopeSingle
)

// Comparator identifies the direction of a clause's threshold check.
type Comparator int

const (
	GTE Comparator = iota + 1
	LTE
)

// Clause is one threshold check within a Goal.
//
// metrics.Def.LowerIsBetter is a display hint only — pace/performance
// clauses are expressed directly as LTE with the right units (e.g.
// "sub-5:00/km" is Metric: Pace, Comparator: LTE, Value: 300), so Evaluate
// never branches on it.
type Clause struct {
	ID         string
	Position   int
	Scope      Scope
	Metric     metrics.Key
	Comparator Comparator
	Value      float64
	// Def, when set (Def.Key != ""), is used in place of metrics.Lookup(Metric)
	// to resolve this clause's Def. This is what lets a clause reference a
	// custom metric: this package is deliberately DB-free and can't look up
	// a custom Def itself (those are built at the service layer — see
	// metrics.NewCustomDef), so the caller resolves it once (via its own
	// merged built-in+custom lookup) and hands the already-built Def
	// straight through. Left unset, behavior is identical to before this
	// field existed — every existing caller and test that only sets Metric
	// keeps resolving via metrics.Lookup exactly as it always has.
	Def metrics.Def
}

// resolveDef returns c's Def: the explicitly-attached one if set, otherwise
// the built-in registry lookup by Metric — the same fallback semantics
// every caller of this package relied on before Def existed.
func resolveDef(c Clause) (metrics.Def, bool) {
	if c.Def.Key != "" {
		return c.Def, true
	}
	return metrics.Lookup(c.Metric)
}

// Goal is one goal's evaluable definition: which activities count
// (Sport, PeriodStart, PeriodEnd) and what must hold of them (Clauses).
type Goal struct {
	ID          string
	Sport       sports.Selector
	PeriodStart int64 // unix seconds, inclusive
	PeriodEnd   int64 // unix seconds, inclusive
	Clauses     []Clause
}

// ClauseResult is one Clause's evaluated outcome.
type ClauseResult struct {
	ClauseID        string
	Position        int
	CurrentValue    float64
	HasValue        bool // false = no activity supplied this metric (nullable) or no distance for pace
	Satisfied       bool
	Ratio           float64 // 0..1, direction-aware, see ratio()
	ConsideredCount int     // activities in sport+period scope
	CountedCount    int     // of those, how many actually carried the metric
}

// Result is a Goal's evaluated outcome.
type Result struct {
	GoalID                string
	Satisfied             bool
	Ratio                 float64        // min over all clause ratios — the weakest link
	Clauses               []ClauseResult // in Position order
	ConsideredCount       int
	QualifyingActivityID  int64 // 0 = none; set when every SINGLE clause is met by one activity
	BestAttemptActivityID int64 // 0 = none; set when SINGLE clauses exist but none fully qualifies
}

// Evaluate assumes nothing about acts beyond it being the caller's full
// activity set for this goal's user — sport and period filtering happen
// HERE, inside Evaluate, which is what makes this function pure and
// independent of how the caller fetched the activities.
//
// A Goal with zero clauses evaluates to Satisfied=true (vacuous truth). This
// is an accepted, documented assumption: Task 7's CreateGoal will require at
// least one clause, so this package does not guard against it.
func Evaluate(g Goal, acts []metrics.Activity) Result {
	filtered := make([]metrics.Activity, 0, len(acts))
	for _, a := range acts {
		if sports.Matches(g.Sport, a.SportType, a.Type) &&
			g.PeriodStart <= a.StartDate && a.StartDate <= g.PeriodEnd {
			filtered = append(filtered, a)
		}
	}
	considered := len(filtered)

	var (
		results               []ClauseResult
		allAggregateSatisfied = true
	)

	var singleClauses []Clause
	for _, c := range g.Clauses {
		switch c.Scope {
		case ScopeAggregate:
			cr := evaluateAggregate(c, filtered, considered)
			if !cr.Satisfied {
				allAggregateSatisfied = false
			}
			results = append(results, cr)
		case ScopeSingle:
			singleClauses = append(singleClauses, c)
		default:
			// Defensive only: Clause.Scope is validated upstream (Task 7).
			// An unrecognized scope (including the zero value) must never be
			// silently dropped — that would let it fail open (falsify
			// nothing, contribute no ratio). Report it as unsatisfied and
			// valueless, same shape as evaluateAggregate's unknown-metric-key
			// fallback, and fail the goal closed.
			results = append(results, ClauseResult{ClauseID: c.ID, Position: c.Position, ConsideredCount: considered})
			allAggregateSatisfied = false
		}
	}

	var qualifyingID, bestAttemptID int64
	if len(singleClauses) > 0 {
		var singleResults []ClauseResult
		singleResults, qualifyingID, bestAttemptID = evaluateSingle(singleClauses, filtered, considered)
		results = append(results, singleResults...)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Position < results[j].Position })

	satisfied := allAggregateSatisfied && (len(singleClauses) == 0 || qualifyingID != 0)

	minRatio := 1.0
	for i, cr := range results {
		if i == 0 || cr.Ratio < minRatio {
			minRatio = cr.Ratio
		}
	}

	return Result{
		GoalID:                g.ID,
		Satisfied:             satisfied,
		Ratio:                 minRatio,
		Clauses:               results,
		ConsideredCount:       considered,
		QualifyingActivityID:  qualifyingID,
		BestAttemptActivityID: bestAttemptID,
	}
}

// evaluateAggregate folds every filtered activity through the clause's
// metric's Accumulator and compares the result against the clause's
// threshold.
func evaluateAggregate(c Clause, filtered []metrics.Activity, considered int) ClauseResult {
	def, ok := resolveDef(c)
	if !ok {
		// Defensive only: Clause.Metric is validated upstream (Task 7).
		// An unknown key must never panic — report it as unsatisfied and
		// valueless rather than crash the evaluator.
		return ClauseResult{ClauseID: c.ID, Position: c.Position, ConsideredCount: considered}
	}

	acc := def.NewAcc()
	for _, a := range filtered {
		acc.Add(a)
	}
	value, counted, hasValue := acc.Result()
	satisfied := hasValue && compare(value, c.Comparator, c.Value)

	return ClauseResult{
		ClauseID:        c.ID,
		Position:        c.Position,
		CurrentValue:    value,
		HasValue:        hasValue,
		Satisfied:       satisfied,
		Ratio:           ratio(hasValue, value, c.Comparator, c.Value),
		ConsideredCount: considered,
		CountedCount:    counted,
	}
}

// singleActivityScore is one filtered activity's evaluation against every
// SINGLE clause on the goal, used both to find a fully-qualifying activity
// and, failing that, to pick the best-attempt activity.
type singleActivityScore struct {
	activity       metrics.Activity
	satisfiedCount int
	meanRatio      float64
}

// evaluateSingle implements the SINGLE-clause evaluation flow: find the
// single activity (if any) that satisfies every SINGLE clause at once, and
// report every SINGLE clause's numbers from that one activity — never
// stitched together from different activities. Failing that, do the same
// from the single best-attempt activity, with every clause reported
// unsatisfied.
func evaluateSingle(clauses []Clause, filtered []metrics.Activity, considered int) (results []ClauseResult, qualifyingID, bestAttemptID int64) {
	defs := make([]metrics.Def, len(clauses))
	for i, c := range clauses {
		d, ok := resolveDef(c)
		if !ok {
			// Defensive only: see evaluateAggregate.
			d = metrics.Def{}
		}
		defs[i] = d
	}

	// countedByClause[i] = how many filtered activities actually carried
	// clause i's metric (i.e. def.Single returned ok=true), independent of
	// whether that activity satisfied the clause or won selection.
	countedByClause := make([]int, len(clauses))

	var (
		haveQualifying bool
		qualifying     metrics.Activity
		haveBest       bool
		best           singleActivityScore
	)

	for _, a := range filtered {
		satisfiedCount := 0
		ratioSum := 0.0
		for i, c := range clauses {
			val, ok := singleValue(defs[i], a)
			if ok {
				countedByClause[i]++
			}
			if ok && compare(val, c.Comparator, c.Value) {
				satisfiedCount++
			}
			ratioSum += ratio(ok, val, c.Comparator, c.Value)
		}
		score := singleActivityScore{
			activity:       a,
			satisfiedCount: satisfiedCount,
			meanRatio:      ratioSum / float64(len(clauses)),
		}

		if satisfiedCount == len(clauses) {
			if !haveQualifying || moreRecent(a, qualifying) {
				haveQualifying = true
				qualifying = a
			}
		}

		if !haveBest || betterAttempt(score, best) {
			haveBest = true
			best = score
		}
	}

	results = make([]ClauseResult, len(clauses))

	switch {
	case haveQualifying:
		qualifyingID = qualifying.StravaActivityID
		for i, c := range clauses {
			val, ok := singleValue(defs[i], qualifying)
			results[i] = ClauseResult{
				ClauseID:        c.ID,
				Position:        c.Position,
				CurrentValue:    val,
				HasValue:        ok,
				Satisfied:       true,
				Ratio:           ratio(ok, val, c.Comparator, c.Value),
				ConsideredCount: considered,
				CountedCount:    countedByClause[i],
			}
		}
	case haveBest:
		bestAttemptID = best.activity.StravaActivityID
		for i, c := range clauses {
			val, ok := singleValue(defs[i], best.activity)
			results[i] = ClauseResult{
				ClauseID:        c.ID,
				Position:        c.Position,
				CurrentValue:    val,
				HasValue:        ok,
				Satisfied:       false,
				Ratio:           ratio(ok, val, c.Comparator, c.Value),
				ConsideredCount: considered,
				CountedCount:    countedByClause[i],
			}
		}
	default:
		// No filtered activities at all: every SINGLE clause is
		// unsatisfied and valueless, with neither a qualifying nor a
		// best-attempt activity to report.
		for i, c := range clauses {
			results[i] = ClauseResult{
				ClauseID:        c.ID,
				Position:        c.Position,
				ConsideredCount: considered,
			}
		}
	}

	return results, qualifyingID, bestAttemptID
}

// singleValue reads a's per-activity value for def, defensively: an unknown
// metric key (see evaluateSingle's Lookup fallback) leaves Single nil, which
// must read as "absent" rather than panic.
func singleValue(def metrics.Def, a metrics.Activity) (value float64, ok bool) {
	if def.Single == nil {
		return 0, false
	}
	return def.Single(a)
}

// moreRecent reports whether a should be preferred over b as the
// most-recently-started activity: StartDate desc, tie-broken by
// StravaActivityID desc for determinism.
func moreRecent(a, b metrics.Activity) bool {
	if a.StartDate != b.StartDate {
		return a.StartDate > b.StartDate
	}
	return a.StravaActivityID > b.StravaActivityID
}

// betterAttempt reports whether a should be preferred over b as the
// best-attempt activity: most SINGLE clauses satisfied, then highest mean
// ratio, then most recent.
func betterAttempt(a, b singleActivityScore) bool {
	if a.satisfiedCount != b.satisfiedCount {
		return a.satisfiedCount > b.satisfiedCount
	}
	if a.meanRatio != b.meanRatio {
		return a.meanRatio > b.meanRatio
	}
	return moreRecent(a.activity, b.activity)
}

// compare applies a Comparator between a current value and a threshold.
func compare(value float64, cmp Comparator, target float64) bool {
	switch cmp {
	case GTE:
		return value >= target
	case LTE:
		return value <= target
	default:
		return false
	}
}

// ratio computes a clause's 0..1 progress ratio.
//
// GTE: clamp(current/target, 0, 1), guarding target <= 0 (ratio = 1 if
// already satisfied else 0). LTE: clamp(target/current, 0, 1) when
// current > 0; when current == 0 and hasValue, ratio = 1 (achieved "at most
// X" with zero). When !hasValue, ratio = 0 regardless of comparator or
// direction — absence is never a pass.
func ratio(hasValue bool, current float64, cmp Comparator, target float64) float64 {
	if !hasValue {
		return 0
	}
	switch cmp {
	case GTE:
		if target <= 0 {
			if current >= target {
				return 1
			}
			return 0
		}
		return clamp(current/target, 0, 1)
	case LTE:
		if current > 0 {
			return clamp(target/current, 0, 1)
		}
		return 1
	default:
		return 0
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
