package progress

import "github.com/fitter/svc/internal/metrics"

// Window is an inclusive [Start, End] slice of time, unix seconds.
type Window struct{ Start, End int64 }

// ForWindow restricts g to w, for leaderboard week/month boards. The goal's
// period is clipped to the window, and every aggregate sum/count clause's
// target is pro-rated by (clipped length / full length) so it reads as "the
// pace needed". Best/max, average and single-activity targets stay whole: a
// 100 kg lift is 100 kg in any week. ok=false when w misses g's period.
func ForWindow(g Goal, w Window) (Goal, bool) {
	start, end := max(g.PeriodStart, w.Start), min(g.PeriodEnd, w.End)
	if end < start || g.PeriodEnd < g.PeriodStart {
		return g, false
	}
	factor := float64(end-start+1) / float64(g.PeriodEnd-g.PeriodStart+1)
	out := g
	out.PeriodStart, out.PeriodEnd = start, end
	out.Clauses = make([]Clause, len(g.Clauses))
	for i, c := range g.Clauses {
		if def, ok := resolveDef(c); ok && c.Scope == ScopeAggregate && isSumLike(def) {
			c.Value *= factor
		}
		out.Clauses[i] = c
	}
	return out, true
}

func isSumLike(d metrics.Def) bool {
	return d.Aggregation == metrics.AggSum || d.Aggregation == metrics.AggCount
}

// WindowExcluded reports whether a windowed result should leave the board:
// nothing was done in the window and the goal has a clause (best, average or
// single-activity) that can't be judged from an empty window. Sum/count-only
// and recurring goals stay: zero is a real, behind-pace score for them.
func WindowExcluded(g Goal, res Result) bool {
	if res.ConsideredCount > 0 || g.Recurrence != RecurNone {
		return false
	}
	for _, c := range g.Clauses {
		if def, ok := resolveDef(c); !ok || c.Scope != ScopeAggregate || !isSumLike(def) {
			return true
		}
	}
	return false
}
