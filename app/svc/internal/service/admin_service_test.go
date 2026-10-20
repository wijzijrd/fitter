package service_test

import (
	"testing"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/service"
)

func TestCreateCustomMetric_NonAdminRejected(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	adminSvc := service.NewAdminService(deps)

	_, err := adminSvc.CreateCustomMetric(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateCustomMetricRequest{
		Key: "bench_press", Label: "Bench press", Unit: "kg", Aggregation: "max",
		SupportsSingleScope: true, SupportsAggregateScope: true,
	}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("CreateCustomMetric(non-admin) error = %v, want PermissionDenied", err)
	}
}

func TestCreateCustomSport_NonAdminRejected(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	adminSvc := service.NewAdminService(deps)

	_, err := adminSvc.CreateCustomSport(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateCustomSportRequest{
		Key: "powerlifting", Label: "Powerlifting",
	}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("CreateCustomSport(non-admin) error = %v, want PermissionDenied", err)
	}
}

func TestListCustomSports_NonAdminRejected(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	adminSvc := service.NewAdminService(deps)
	if _, err := adminSvc.ListCustomSports(ctxAs(alice.ID), connect.NewRequest(&fitterv1.ListCustomSportsRequest{})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("ListCustomSports(non-admin) error = %v, want PermissionDenied", err)
	}
}

func TestListCustomMetrics_NonAdminRejected(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	adminSvc := service.NewAdminService(deps)
	if _, err := adminSvc.ListCustomMetrics(ctxAs(alice.ID), connect.NewRequest(&fitterv1.ListCustomMetricsRequest{})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("ListCustomMetrics(non-admin) error = %v, want PermissionDenied", err)
	}
}

func TestDeleteCustomMetric_NonAdminRejected(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	adminSvc := service.NewAdminService(deps)
	if _, err := adminSvc.DeleteCustomMetric(ctxAs(alice.ID), connect.NewRequest(&fitterv1.DeleteCustomMetricRequest{Id: "whatever"})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("DeleteCustomMetric(non-admin) error = %v, want PermissionDenied", err)
	}
}

func TestDeleteCustomSport_NonAdminRejected(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	adminSvc := service.NewAdminService(deps)
	if _, err := adminSvc.DeleteCustomSport(ctxAs(alice.ID), connect.NewRequest(&fitterv1.DeleteCustomSportRequest{Id: "whatever"})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("DeleteCustomSport(non-admin) error = %v, want PermissionDenied", err)
	}
}

func TestCreateCustomMetric_KeyCollisions(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	adminSvc := service.NewAdminService(deps)
	ctx := ctxAsAdmin(admin.ID)

	// Collides with a built-in metrics.Key.
	_, err := adminSvc.CreateCustomMetric(ctx, connect.NewRequest(&fitterv1.CreateCustomMetricRequest{
		Key: "distance", Label: "Distance again", Unit: "m", Aggregation: "sum",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateCustomMetric(built-in key collision) error = %v, want InvalidArgument", err)
	}

	// First creation succeeds.
	if _, err := adminSvc.CreateCustomMetric(ctx, connect.NewRequest(&fitterv1.CreateCustomMetricRequest{
		Key: "bench_press", Label: "Bench press", Unit: "kg", Aggregation: "max",
		SupportsSingleScope: true, SupportsAggregateScope: true,
	})); err != nil {
		t.Fatalf("CreateCustomMetric(bench_press): %v", err)
	}

	// A second custom metric with the same key collides too.
	_, err = adminSvc.CreateCustomMetric(ctx, connect.NewRequest(&fitterv1.CreateCustomMetricRequest{
		Key: "bench_press", Label: "Bench press dupe", Unit: "kg", Aggregation: "max",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateCustomMetric(duplicate custom key) error = %v, want InvalidArgument", err)
	}
}

func TestCreateCustomMetric_UnknownAggregationRejected(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	adminSvc := service.NewAdminService(deps)

	_, err := adminSvc.CreateCustomMetric(ctxAsAdmin(admin.ID), connect.NewRequest(&fitterv1.CreateCustomMetricRequest{
		Key: "bench_press", Label: "Bench press", Unit: "kg", Aggregation: "derived",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateCustomMetric(unknown aggregation) error = %v, want InvalidArgument", err)
	}
}

func TestCreateCustomMetric_UnknownSportRefRejected(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	adminSvc := service.NewAdminService(deps)

	_, err := adminSvc.CreateCustomMetric(ctxAsAdmin(admin.ID), connect.NewRequest(&fitterv1.CreateCustomMetricRequest{
		Key: "bench_press", Label: "Bench press", Unit: "kg", Aggregation: "max",
		ApplicableSportRefs: []string{"not_a_real_sport_or_group"},
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateCustomMetric(unknown sport ref) error = %v, want InvalidArgument", err)
	}
}

func TestCreateCustomSport_CollidesWithRealStravaType(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	adminSvc := service.NewAdminService(deps)

	_, err := adminSvc.CreateCustomSport(ctxAsAdmin(admin.ID), connect.NewRequest(&fitterv1.CreateCustomSportRequest{
		Key: "Run", Label: "Not actually custom",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateCustomSport(real Strava type) error = %v, want InvalidArgument", err)
	}
}

func TestCreateCustomSport_DuplicateKeyRejected(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	adminSvc := service.NewAdminService(deps)
	ctx := ctxAsAdmin(admin.ID)

	if _, err := adminSvc.CreateCustomSport(ctx, connect.NewRequest(&fitterv1.CreateCustomSportRequest{Key: "powerlifting", Label: "Powerlifting"})); err != nil {
		t.Fatalf("CreateCustomSport: %v", err)
	}
	_, err := adminSvc.CreateCustomSport(ctx, connect.NewRequest(&fitterv1.CreateCustomSportRequest{Key: "powerlifting", Label: "Dupe"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateCustomSport(duplicate) error = %v, want InvalidArgument", err)
	}
}

func TestAdminCustomSportMetric_CreateListDelete(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	adminSvc := service.NewAdminService(deps)
	ctx := ctxAsAdmin(admin.ID)

	sportResp, err := adminSvc.CreateCustomSport(ctx, connect.NewRequest(&fitterv1.CreateCustomSportRequest{
		Key: "powerlifting", Label: "Powerlifting",
	}))
	if err != nil {
		t.Fatalf("CreateCustomSport: %v", err)
	}
	if sportResp.Msg.GetSport().GetKey() != "powerlifting" {
		t.Fatalf("CreateCustomSport: key = %q, want powerlifting", sportResp.Msg.GetSport().GetKey())
	}

	metricResp, err := adminSvc.CreateCustomMetric(ctx, connect.NewRequest(&fitterv1.CreateCustomMetricRequest{
		Key: "bench_press", Label: "Bench press", Unit: "kg", Aggregation: "max",
		Nullable: true, SupportsSingleScope: true, SupportsAggregateScope: true,
		ApplicableSportRefs: []string{"strength", "powerlifting"},
	}))
	if err != nil {
		t.Fatalf("CreateCustomMetric: %v", err)
	}
	created := metricResp.Msg.GetMetric()
	if created.GetKey() != "bench_press" || created.GetUnit() != "kg" || created.GetAggregation() != "max" {
		t.Fatalf("CreateCustomMetric: got %+v", created)
	}
	if len(created.GetApplicableSportRefs()) != 2 {
		t.Fatalf("CreateCustomMetric: applicable_sport_refs = %v, want 2 entries", created.GetApplicableSportRefs())
	}

	listSportsResp, err := adminSvc.ListCustomSports(ctx, connect.NewRequest(&fitterv1.ListCustomSportsRequest{}))
	if err != nil {
		t.Fatalf("ListCustomSports: %v", err)
	}
	if len(listSportsResp.Msg.GetSports()) != 1 {
		t.Fatalf("ListCustomSports: got %d, want 1", len(listSportsResp.Msg.GetSports()))
	}

	listMetricsResp, err := adminSvc.ListCustomMetrics(ctx, connect.NewRequest(&fitterv1.ListCustomMetricsRequest{}))
	if err != nil {
		t.Fatalf("ListCustomMetrics: %v", err)
	}
	if len(listMetricsResp.Msg.GetMetrics()) != 1 {
		t.Fatalf("ListCustomMetrics: got %d, want 1", len(listMetricsResp.Msg.GetMetrics()))
	}
	if len(listMetricsResp.Msg.GetMetrics()[0].GetApplicableSportRefs()) != 2 {
		t.Fatalf("ListCustomMetrics: applicable_sport_refs not round-tripped: %+v", listMetricsResp.Msg.GetMetrics()[0])
	}

	// Delete both, confirm they're gone.
	if _, err := adminSvc.DeleteCustomMetric(ctx, connect.NewRequest(&fitterv1.DeleteCustomMetricRequest{Id: created.GetId()})); err != nil {
		t.Fatalf("DeleteCustomMetric: %v", err)
	}
	listMetricsResp, err = adminSvc.ListCustomMetrics(ctx, connect.NewRequest(&fitterv1.ListCustomMetricsRequest{}))
	if err != nil {
		t.Fatalf("ListCustomMetrics after delete: %v", err)
	}
	if len(listMetricsResp.Msg.GetMetrics()) != 0 {
		t.Fatalf("ListCustomMetrics after delete: got %d, want 0", len(listMetricsResp.Msg.GetMetrics()))
	}

	if _, err := adminSvc.DeleteCustomSport(ctx, connect.NewRequest(&fitterv1.DeleteCustomSportRequest{Id: sportResp.Msg.GetSport().GetId()})); err != nil {
		t.Fatalf("DeleteCustomSport: %v", err)
	}
	listSportsResp, err = adminSvc.ListCustomSports(ctx, connect.NewRequest(&fitterv1.ListCustomSportsRequest{}))
	if err != nil {
		t.Fatalf("ListCustomSports after delete: %v", err)
	}
	if len(listSportsResp.Msg.GetSports()) != 0 {
		t.Fatalf("ListCustomSports after delete: got %d, want 0", len(listSportsResp.Msg.GetSports()))
	}
}

func TestDeleteCustomMetric_NotFound(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	adminSvc := service.NewAdminService(deps)
	_, err := adminSvc.DeleteCustomMetric(ctxAsAdmin(admin.ID), connect.NewRequest(&fitterv1.DeleteCustomMetricRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteCustomMetric(missing) error = %v, want NotFound", err)
	}
}

// TestListMetrics_IncludesAdminCreatedCustomMetric is the Task 4 brief's
// required "an admin-created custom metric appears in ListMetrics" case —
// CatalogService.ListMetrics is public/DB-free-looking from the wire's
// point of view (still in interceptor.go's exemptProcedures), but now reads
// through to custom_metrics too.
func TestListMetrics_IncludesAdminCreatedCustomMetric(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	adminSvc := service.NewAdminService(deps)
	catalogSvc := service.NewCatalogService(deps)

	if _, err := adminSvc.CreateCustomMetric(ctxAsAdmin(admin.ID), connect.NewRequest(&fitterv1.CreateCustomMetricRequest{
		Key: "bench_press", Label: "Bench press", Unit: "kg", Aggregation: "max",
		Nullable: true, SupportsSingleScope: true, SupportsAggregateScope: true,
		ApplicableSportRefs: []string{"strength"},
	})); err != nil {
		t.Fatalf("CreateCustomMetric: %v", err)
	}

	resp, err := catalogSvc.ListMetrics(ctxAs(admin.ID), connect.NewRequest(&fitterv1.ListMetricsRequest{}))
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	metrics := resp.Msg.GetMetrics()
	if len(metrics) != 18 { // 17 built-in + 1 custom
		t.Fatalf("ListMetrics: got %d metrics, want 18", len(metrics))
	}
	var found *fitterv1.MetricInfo
	for _, m := range metrics {
		if m.GetKey() == "bench_press" {
			found = m
		}
	}
	if found == nil {
		t.Fatal("ListMetrics: custom metric bench_press not found")
	}
	if found.GetLabel() != "Bench press" {
		t.Fatalf("ListMetrics: bench_press label = %q, want %q", found.GetLabel(), "Bench press")
	}
	if found.GetMetric() != fitterv1.ActivityMetric_ACTIVITY_METRIC_UNSPECIFIED {
		t.Fatalf("ListMetrics: bench_press ActivityMetric = %v, want UNSPECIFIED (custom metrics have no fixed enum value)", found.GetMetric())
	}
	if len(found.GetApplicableSportGroups()) != 1 || found.GetApplicableSportGroups()[0] != "strength" {
		t.Fatalf("ListMetrics: bench_press applicable_sport_groups = %v, want [strength]", found.GetApplicableSportGroups())
	}
}

// TestListMetrics_BuiltinApplicability spot-checks the curated built-in
// applicability map is actually wired through ListMetrics.
func TestListMetrics_BuiltinApplicability(t *testing.T) {
	deps := newDeps(t)
	catalogSvc := service.NewCatalogService(deps)

	resp, err := catalogSvc.ListMetrics(ctxAs("nobody"), connect.NewRequest(&fitterv1.ListMetricsRequest{}))
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	byKey := make(map[string]*fitterv1.MetricInfo, len(resp.Msg.GetMetrics()))
	for _, m := range resp.Msg.GetMetrics() {
		byKey[m.GetKey()] = m
	}

	if groups := byKey["distance"].GetApplicableSportGroups(); len(groups) == 0 {
		t.Fatal("distance: applicable_sport_groups is empty, want a curated restriction (strength excluded)")
	} else {
		for _, g := range groups {
			if g == "strength" {
				t.Fatal("distance: strength should not be in applicable_sport_groups")
			}
		}
	}
	if groups := byKey["moving_time"].GetApplicableSportGroups(); len(groups) != 0 {
		t.Fatalf("moving_time: applicable_sport_groups = %v, want empty (applies to every sport)", groups)
	}
	if groups := byKey["average_watts"].GetApplicableSportGroups(); len(groups) != 1 || groups[0] != "ride" {
		t.Fatalf("average_watts: applicable_sport_groups = %v, want [ride]", groups)
	}
}
