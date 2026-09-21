package service

import (
	"context"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/sports"
)

// ListMetrics maps internal/metrics.All() to the wire MetricInfo list. Pure
// translation, no DB access — the login-time onboarding UI needs this
// before a user necessarily has a session token (see the auth interceptor's
// exemptProcedures).
func (s *CatalogService) ListMetrics(ctx context.Context, req *connect.Request[fitterv1.ListMetricsRequest]) (*connect.Response[fitterv1.ListMetricsResponse], error) {
	defs := metrics.All()
	out := make([]*fitterv1.MetricInfo, len(defs))
	for i, d := range defs {
		out[i] = &fitterv1.MetricInfo{
			Metric:                 keyToMetric(d.Key),
			Key:                    string(d.Key),
			Label:                  d.Label,
			Unit:                   unitToProto(d.Unit),
			Aggregation:            aggregationToProto(d.Aggregation),
			Nullable:               d.Nullable,
			SupportsSingleScope:    d.SupportsSingle,
			SupportsAggregateScope: d.SupportsAggregate,
			LowerIsBetter:          d.LowerIsBetter,
		}
	}
	return connect.NewResponse(&fitterv1.ListMetricsResponse{Metrics: out}), nil
}

// ListSports maps internal/sports.Groups() to the wire SportGroupInfo list.
// Pure translation, no DB access.
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
