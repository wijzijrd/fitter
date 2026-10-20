package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/metrics"
)

// aggregationByDB translates custom_metrics.aggregation's four allowed
// string values (DB CHECK-constrained to sum/average/max/count — never
// "derived", which only Pace uses and which no generic factory can express)
// to internal/metrics.Aggregation. The proto wire form keeps the raw DB
// string (CustomMetric.aggregation), so no inverse map back to a string is
// needed — metrics.Aggregation already has its own proto translation
// (aggregationToProto) for CatalogService.ListMetrics.
var aggregationByDB = map[string]metrics.Aggregation{
	"sum":     metrics.AggSum,
	"average": metrics.AggWeightedAvg,
	"max":     metrics.AggMax,
	"count":   metrics.AggCount,
}

// customDefFromRow builds a metrics.Def for one persisted custom_metrics
// row via metrics.NewCustomDef, the factory internal/metrics gained for
// exactly this purpose. ok is false only in the defensive case of an
// aggregation value the DB's own CHECK constraint should never have let
// through.
func customDefFromRow(m db.CustomMetric) (metrics.Def, bool) {
	agg, ok := aggregationByDB[m.Aggregation]
	if !ok {
		return metrics.Def{}, false
	}
	return metrics.NewCustomDef(
		metrics.Key(m.Key), m.Label, m.Unit, agg,
		m.Nullable, m.LowerIsBetter,
		m.SupportsSingleScope, m.SupportsAggregateScope,
	), true
}

// lookupMetric is the merged built-in+custom metric lookup: every
// service-layer call site that used to call metrics.Lookup directly now
// calls this instead, so a goal_clause referencing a custom metric resolves
// correctly everywhere a built-in one does (leaderboard, timeline, goal
// clause validation, clause-to-progress conversion). Built-ins are checked
// first — the overwhelmingly common case — so no DB hit is paid for them.
//
// q is taken explicitly (a free function, not a *Deps method) rather than
// always reading d.Queries: some callers (createGoalInTx) run inside an
// already-open write transaction on qtx, and dbtest pins the test DB to a
// single pooled connection (see dbtest.New's doc comment) — querying
// through the ambient d.Queries while that transaction still holds the only
// connection would deadlock. Every caller passes whichever *db.Queries is
// actually safe to use at that point: qtx inside a transaction, d.Queries
// (or s.Queries, via the embedded *Deps) otherwise.
//
// The third return value is a genuine DB error (as opposed to "not found",
// which is ok=false, err=nil) — callers that previously only had a bool to
// check must now also propagate this.
func lookupMetric(ctx context.Context, q *db.Queries, key metrics.Key) (metrics.Def, bool, error) {
	if def, ok := metrics.Lookup(key); ok {
		return def, true, nil
	}
	row, err := q.GetCustomMetricByKey(ctx, string(key))
	if errors.Is(err, sql.ErrNoRows) {
		return metrics.Def{}, false, nil
	}
	if err != nil {
		return metrics.Def{}, false, err
	}
	def, ok := customDefFromRow(row)
	if !ok {
		return metrics.Def{}, false, nil
	}
	return def, true, nil
}
