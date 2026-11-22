package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/sports"
	"github.com/google/uuid"
)

// isKnownStravaSportType reports whether t names a real Strava sport_type
// already curated into internal/sports' group table. It reads only via
// sports.Groups() (the package's existing exported accessor) — no change to
// internal/sports itself, per the Global Constraints: custom sports need
// zero changes there.
func isKnownStravaSportType(t string) bool {
	for _, g := range sports.Groups() {
		for _, st := range g.SportTypes {
			if st == t {
				return true
			}
		}
	}
	return false
}

// isKnownSportRef reports whether ref is a valid target for a custom
// metric's applicability list: either a curated built-in sports.Group key
// (e.g. "strength") or an existing custom_sports.key. No FK enforces this
// (see the migration's comment on custom_metric_sports) — it's validated
// here at the service boundary instead, mirroring how goal_clauses.metric
// already has no CHECK by deliberate design.
func (d *Deps) isKnownSportRef(ctx context.Context, ref string) (bool, error) {
	if sports.ValidGroup(ref) {
		return true, nil
	}
	_, err := d.Queries.GetCustomSportByKey(ctx, ref)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func customSportToProto(s db.CustomSport) *fitterv1.CustomSport {
	return &fitterv1.CustomSport{
		Id:        s.ID,
		Key:       s.Key,
		Label:     s.Label,
		CreatedBy: s.CreatedBy,
		CreatedAt: unixToTimestamp(s.CreatedAt),
	}
}

func customMetricToProto(m db.CustomMetric, sportRefs []string) *fitterv1.CustomMetric {
	return &fitterv1.CustomMetric{
		Id:                     m.ID,
		Key:                    m.Key,
		Label:                  m.Label,
		Unit:                   m.Unit,
		Aggregation:            m.Aggregation,
		Nullable:               m.Nullable,
		LowerIsBetter:          m.LowerIsBetter,
		SupportsSingleScope:    m.SupportsSingleScope,
		SupportsAggregateScope: m.SupportsAggregateScope,
		ApplicableSportRefs:    sportRefs,
		CreatedBy:              m.CreatedBy,
		CreatedAt:              unixToTimestamp(m.CreatedAt),
	}
}

const (
	maxSportKeyLen   = 64
	maxSportLabelLen = 60
)

// validateSportKeyAndLabel returns the label trimmed. A key is matched verbatim against
// activities' sport_type and shares a namespace with the group keys in custom-metric sport refs,
// so it must be a clean token that can't shadow a group; a label is display text.
func validateSportKeyAndLabel(key, label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" || utf8.RuneCountInString(label) > maxSportLabelLen {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("label is required and at most %d characters", maxSportLabelLen))
	}
	if key == "" || utf8.RuneCountInString(key) > maxSportKeyLen {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("key is required and at most %d characters", maxSportKeyLen))
	}
	if strings.ContainsFunc(key, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("key can't contain spaces or control characters"))
	}
	if sports.ValidGroup(key) {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("key %q is a built-in sport group", key))
	}
	return label, nil
}

func (s *AdminService) CreateCustomSport(ctx context.Context, req *connect.Request[fitterv1.CreateCustomSportRequest]) (*connect.Response[fitterv1.CreateCustomSportResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}

	key := req.Msg.GetKey()
	label, err := validateSportKeyAndLabel(key, req.Msg.GetLabel())
	if err != nil {
		return nil, err
	}
	if isKnownStravaSportType(key) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("key %q collides with a real Strava sport_type", key))
	}
	if _, err := s.Queries.GetCustomSportByKey(ctx, key); err == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("key %q is already in use by a custom sport", key))
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	qtx := s.Queries.WithTx(tx)
	row, err := qtx.CreateCustomSport(ctx, db.CreateCustomSportParams{
		ID:        uuid.NewString(),
		Key:       key,
		Label:     label,
		CreatedBy: userID,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := audit(ctx, qtx, userID, "create_sport", row.ID, key); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.CreateCustomSportResponse{Sport: customSportToProto(row)}), nil
}

func (s *AdminService) ListCustomSports(ctx context.Context, req *connect.Request[fitterv1.ListCustomSportsRequest]) (*connect.Response[fitterv1.ListCustomSportsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := s.Queries.ListCustomSports(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*fitterv1.CustomSport, len(rows))
	for i, r := range rows {
		out[i] = customSportToProto(r)
	}
	return connect.NewResponse(&fitterv1.ListCustomSportsResponse{Sports: out}), nil
}

func (s *AdminService) DeleteCustomSport(ctx context.Context, req *connect.Request[fitterv1.DeleteCustomSportRequest]) (*connect.Response[fitterv1.DeleteCustomSportResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	actor, _ := auth.UserIDFromContext(ctx)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	qtx := s.Queries.WithTx(tx)
	sp, err := qtx.GetCustomSportByID(ctx, req.Msg.GetId())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("custom sport not found"))
	} else if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// No refs means "applies to every sport", so deleting a sport a metric is scoped to would
	// silently widen that metric. Remove the metric (or its scope) first.
	if n, err := qtx.CountMetricsScopedToSport(ctx, sp.Key); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if n > 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%d custom metric(s) are scoped to this sport; delete them first", n))
	}
	if err := qtx.DeleteCustomSport(ctx, sp.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := audit(ctx, qtx, actor, "delete_sport", sp.ID, sp.Key); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.DeleteCustomSportResponse{}), nil
}

func (s *AdminService) CreateCustomMetric(ctx context.Context, req *connect.Request[fitterv1.CreateCustomMetricRequest]) (*connect.Response[fitterv1.CreateCustomMetricResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}

	msg := req.Msg
	key := msg.GetKey()
	label := msg.GetLabel()
	if key == "" || label == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key and label are required"))
	}
	if _, ok := metrics.Lookup(metrics.Key(key)); ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("key %q collides with a built-in metric", key))
	}
	if _, err := s.Queries.GetCustomMetricByKey(ctx, key); err == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("key %q is already in use by a custom metric", key))
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if _, ok := aggregationByDB[msg.GetAggregation()]; !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown aggregation %q: must be one of sum, average, max, count", msg.GetAggregation()))
	}
	for _, ref := range msg.GetApplicableSportRefs() {
		known, err := s.isKnownSportRef(ctx, ref)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !known {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown sport reference %q: must be a curated sport group or an existing custom sport key", ref))
		}
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	qtx := s.Queries.WithTx(tx)

	row, err := qtx.CreateCustomMetric(ctx, db.CreateCustomMetricParams{
		ID:                     uuid.NewString(),
		Key:                    key,
		Label:                  label,
		Unit:                   msg.GetUnit(),
		Aggregation:            msg.GetAggregation(),
		Nullable:               msg.GetNullable(),
		LowerIsBetter:          msg.GetLowerIsBetter(),
		SupportsSingleScope:    msg.GetSupportsSingleScope(),
		SupportsAggregateScope: msg.GetSupportsAggregateScope(),
		CreatedBy:              userID,
		CreatedAt:              time.Now().Unix(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, ref := range msg.GetApplicableSportRefs() {
		if err := qtx.CreateCustomMetricSport(ctx, db.CreateCustomMetricSportParams{MetricID: row.ID, SportRef: ref}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := audit(ctx, qtx, userID, "create_metric", row.ID, key); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&fitterv1.CreateCustomMetricResponse{
		Metric: customMetricToProto(row, msg.GetApplicableSportRefs()),
	}), nil
}

func (s *AdminService) ListCustomMetrics(ctx context.Context, req *connect.Request[fitterv1.ListCustomMetricsRequest]) (*connect.Response[fitterv1.ListCustomMetricsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := s.Queries.ListCustomMetrics(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	sportRefsByMetric, err := s.sportRefsByCustomMetric(ctx, rows)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*fitterv1.CustomMetric, len(rows))
	for i, r := range rows {
		out[i] = customMetricToProto(r, sportRefsByMetric[r.ID])
	}
	return connect.NewResponse(&fitterv1.ListCustomMetricsResponse{Metrics: out}), nil
}

// sportRefsByCustomMetric batch-fetches every applicability row for rows in
// one query (ListCustomMetricSportsForMetrics), instead of one query per
// custom metric — shared by ListCustomMetrics and CatalogService.ListMetrics.
func (d *Deps) sportRefsByCustomMetric(ctx context.Context, rows []db.CustomMetric) (map[string][]string, error) {
	out := make(map[string][]string, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	sportRows, err := d.Queries.ListCustomMetricSportsForMetrics(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, sr := range sportRows {
		out[sr.MetricID] = append(out[sr.MetricID], sr.SportRef)
	}
	return out, nil
}

func (s *AdminService) DeleteCustomMetric(ctx context.Context, req *connect.Request[fitterv1.DeleteCustomMetricRequest]) (*connect.Response[fitterv1.DeleteCustomMetricResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	actor, _ := auth.UserIDFromContext(ctx)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	qtx := s.Queries.WithTx(tx)
	m, err := qtx.GetCustomMetricByID(ctx, req.Msg.GetId())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("custom metric not found"))
	} else if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// custom_metric_sports rows cascade-delete via their FK's ON DELETE
	// CASCADE (foreign_keys pragma is on for both the real DB and dbtest).
	if err := qtx.DeleteCustomMetric(ctx, m.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := audit(ctx, qtx, actor, "delete_metric", m.ID, m.Key); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.DeleteCustomMetricResponse{}), nil
}
