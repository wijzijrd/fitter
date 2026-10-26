package service

import (
	"context"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/sports"
)

// ListMetrics maps internal/metrics.All() plus every persisted custom
// metric to the wire MetricInfo list — this is the "merge built-in +
// custom" step from the Global Constraints: an admin-created custom metric
// appears in this list (and therefore every dropdown built from it)
// automatically, with no separate frontend fetch. Custom metrics have no
// fixed ActivityMetric enum value (that enum is closed and versioned with
// the 17 built-ins), so MetricInfo.metric is left UNSPECIFIED for them —
// MetricInfo.key is their canonical, stable identifier, exactly as
// documented on the key field.
//
// This RPC is DB-backed now (it wasn't before custom metrics existed —
// see exemptProcedures in internal/auth/interceptor.go, which still
// exempts it from auth: reading the public metric taxonomy needs no
// session, custom metrics included).
func (s *CatalogService) ListMetrics(ctx context.Context, req *connect.Request[fitterv1.ListMetricsRequest]) (*connect.Response[fitterv1.ListMetricsResponse], error) {
	defs := metrics.All()
	out := make([]*fitterv1.MetricInfo, 0, len(defs))
	for _, d := range defs {
		out = append(out, &fitterv1.MetricInfo{
			Metric:                 keyToMetric(d.Key),
			Key:                    string(d.Key),
			Label:                  d.Label,
			Unit:                   unitToProto(d.Unit),
			Aggregation:            aggregationToProto(d.Aggregation),
			Nullable:               d.Nullable,
			SupportsSingleScope:    d.SupportsSingle,
			SupportsAggregateScope: d.SupportsAggregate,
			LowerIsBetter:          d.LowerIsBetter,
			ApplicableSportGroups:  metrics.ApplicableSportGroups(d.Key),
		})
	}

	customRows, err := s.Queries.ListCustomMetrics(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	sportRefsByMetric, err := s.sportRefsByCustomMetric(ctx, customRows)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, r := range customRows {
		def, ok := customDefFromRow(r)
		if !ok {
			// Defensive only: aggregation is DB-CHECK-constrained to a
			// value customDefFromRow always recognizes.
			continue
		}
		out = append(out, &fitterv1.MetricInfo{
			Metric:                 fitterv1.ActivityMetric_ACTIVITY_METRIC_UNSPECIFIED,
			Key:                    string(def.Key),
			Label:                  def.Label,
			Unit:                   fitterv1.MetricUnit_METRIC_UNIT_UNSPECIFIED, // free-text unit (r.Unit) has no closed-enum slot; AdminService.ListCustomMetrics carries it verbatim
			Aggregation:            aggregationToProto(def.Aggregation),
			Nullable:               def.Nullable,
			SupportsSingleScope:    def.SupportsSingle,
			SupportsAggregateScope: def.SupportsAggregate,
			LowerIsBetter:          def.LowerIsBetter,
			ApplicableSportGroups:  sportRefsByMetric[r.ID],
		})
	}

	return connect.NewResponse(&fitterv1.ListMetricsResponse{Metrics: out}), nil
}

// ListSports maps internal/sports.Groups() to the wire SportGroupInfo list.
// Pure translation, no DB access.
//
// Deliberately NOT merged with custom_sports: SportGroupInfo represents a
// curated GROUP (a bucket of exact sport_type strings validated via
// sports.ValidGroup on write — see sportSelectorToDB), and a custom sport
// is not a group, it's a single exact sport_type key. Folding a custom
// sport in here as a pseudo-group would let a client emit
// SportSelector.group = <custom key>, which sportSelectorToDB would then
// reject (sports.ValidGroup only recognizes the 9 curated groups) —
// exactly backwards from how a custom sport is meant to be selected.
// Confirmed by reading the frontend (SportSelectorField.vue): it already
// has a free-text "Specific sport type…" field for the SportSelector.sport_type
// oneof branch, which needs no catalog data at all and already accepts any
// string — including a custom sport's key — with zero further plumbing.
// Custom sports are exposed to the admin panel via AdminService.ListCustomSports
// instead.
func (s *CatalogService) ListSports(ctx context.Context, req *connect.Request[fitterv1.ListSportsRequest]) (*connect.Response[fitterv1.ListSportsResponse], error) {
	defs := sports.Groups()
	out := make([]*fitterv1.SportGroupInfo, len(defs))
	for i, d := range defs {
		out[i] = &fitterv1.SportGroupInfo{
			Group:      string(d.Group),
			Label:      d.Label,
			SportTypes: d.SportTypes,
		}
	}
	return connect.NewResponse(&fitterv1.ListSportsResponse{Groups: out}), nil
}
