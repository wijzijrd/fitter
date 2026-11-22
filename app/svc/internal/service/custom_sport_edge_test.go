package service_test

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/service"
)

func createSport(svc *service.AdminService, admin string, key, label string) (*fitterv1.CustomSport, error) {
	r, err := svc.CreateCustomSport(ctxAsAdmin(admin), connect.NewRequest(&fitterv1.CreateCustomSportRequest{Key: key, Label: label}))
	if err != nil {
		return nil, err
	}
	return r.Msg.GetSport(), nil
}

func auditCount(t *testing.T, deps *service.Deps, action string) int {
	t.Helper()
	var n int
	if err := deps.DB.QueryRow(`SELECT count(*) FROM admin_actions WHERE action = $1`, action).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A custom sport's key is matched verbatim against activities' sport_type, and doubles as a
// custom-metric sport ref next to the group keys, so it has to be a clean token that can't
// shadow a group.
func TestCreateCustomSport_RejectsUnusableKeys(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	svc := service.NewAdminService(deps)

	bad := map[string]string{
		"empty":                        "",
		"only spaces":                  "   ",
		"inner space":                  "power lifting",
		"leading space":                " hyrox",
		"trailing space":               "hyrox ",
		"tab":                          "hy\trox",
		"newline":                      "hy\nrox",
		"too long":                     strings.Repeat("a", 65),
		"collides with a group key":    "run",
		"collides with strength":       "strength",
		"collides with circuits":       "circuits",
		"collides with other":          "other",
		"collides with a Strava type":  "TrailRun",
		"collides with a legacy label": "Crossfit",
	}
	for name, key := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := createSport(svc, admin.ID, key, "Label"); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("key %q: got %v, want InvalidArgument", key, err)
			}
		})
	}
	if n := auditCount(t, deps, "create_sport"); n != 0 {
		t.Fatalf("%d audit rows after only rejected creates, want 0", n)
	}
}

func TestCreateCustomSport_RejectsUnusableLabels(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	svc := service.NewAdminService(deps)
	for name, label := range map[string]string{"empty": "", "spaces only": "   ", "tab only": "\t", "too long": strings.Repeat("é", 61)} {
		t.Run(name, func(t *testing.T) {
			if _, err := createSport(svc, admin.ID, "hyrox", label); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("label %q: got %v, want InvalidArgument", label, err)
			}
		})
	}
}

func TestCreateCustomSport_AcceptsReasonableKeysAndTrimsLabel(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	svc := service.NewAdminService(deps)
	for _, key := range []string{"Hyrox", "hyrox-pro", "pilates_2", "x", strings.Repeat("a", 64), "Pölsa"} {
		s, err := createSport(svc, admin.ID, key, "  Spaced label  ")
		if err != nil {
			t.Fatalf("key %q: %v", key, err)
		}
		if s.GetKey() != key || s.GetLabel() != "Spaced label" {
			t.Fatalf("key %q stored as key=%q label=%q, want the key unchanged and the label trimmed", key, s.GetKey(), s.GetLabel())
		}
	}
}

func TestCreateCustomSport_IsAuditedOncePerSuccess(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	svc := service.NewAdminService(deps)
	if _, err := createSport(svc, admin.ID, "hyrox", "Hyrox"); err != nil {
		t.Fatal(err)
	}
	if _, err := createSport(svc, admin.ID, "hyrox", "Again"); err == nil {
		t.Fatal("duplicate accepted")
	}
	if n := auditCount(t, deps, "create_sport"); n != 1 {
		t.Fatalf("create_sport audit rows = %d, want 1", n)
	}
}

// Deleting a sport that a custom metric is scoped to would silently widen the metric to every
// sport (no refs means "applies everywhere"), so it is refused until the metric is gone.
func TestDeleteCustomSport_RefusedWhileAMetricIsScopedToIt(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	svc := service.NewAdminService(deps)
	ctx := ctxAsAdmin(admin.ID)

	sp, err := createSport(svc, admin.ID, "hyrox", "Hyrox")
	if err != nil {
		t.Fatal(err)
	}
	m, err := svc.CreateCustomMetric(ctx, connect.NewRequest(&fitterv1.CreateCustomMetricRequest{
		Key: "sled_kg", Label: "Sled", Unit: "kg", Aggregation: "max", SupportsAggregateScope: true,
		ApplicableSportRefs: []string{"hyrox"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeleteCustomSport(ctx, connect.NewRequest(&fitterv1.DeleteCustomSportRequest{Id: sp.GetId()})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("delete while referenced: got %v, want FailedPrecondition", err)
	}
	if n := auditCount(t, deps, "delete_sport"); n != 0 {
		t.Fatalf("refused delete left %d audit rows", n)
	}
	if _, err := svc.DeleteCustomMetric(ctx, connect.NewRequest(&fitterv1.DeleteCustomMetricRequest{Id: m.Msg.GetMetric().GetId()})); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeleteCustomSport(ctx, connect.NewRequest(&fitterv1.DeleteCustomSportRequest{Id: sp.GetId()})); err != nil {
		t.Fatalf("delete after the metric is gone: %v", err)
	}
	// Free again once deleted.
	if _, err := createSport(svc, admin.ID, "hyrox", "Hyrox again"); err != nil {
		t.Fatalf("recreate after delete: %v", err)
	}
}

func TestDeleteCustomSport_MissingEmptyAndTwice(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	svc := service.NewAdminService(deps)
	ctx := ctxAsAdmin(admin.ID)
	for _, id := range []string{"", "nope"} {
		if _, err := svc.DeleteCustomSport(ctx, connect.NewRequest(&fitterv1.DeleteCustomSportRequest{Id: id})); connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("delete id %q: got %v, want NotFound", id, err)
		}
	}
	sp, _ := createSport(svc, admin.ID, "hyrox", "Hyrox")
	if _, err := svc.DeleteCustomSport(ctx, connect.NewRequest(&fitterv1.DeleteCustomSportRequest{Id: sp.GetId()})); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeleteCustomSport(ctx, connect.NewRequest(&fitterv1.DeleteCustomSportRequest{Id: sp.GetId()})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("second delete: got %v, want NotFound", err)
	}
	if n := auditCount(t, deps, "delete_sport"); n != 1 {
		t.Fatalf("delete_sport audit rows = %d, want 1", n)
	}
}

// End to end: an admin adds a sport, a member's goal selects it by key, and only activities of
// exactly that sport_type count toward it.
func TestCustomSport_GoalOnItsKeyCountsOnlyThatSport(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	alice := seedUser(t, deps.Queries, "alice")
	if _, err := createSport(service.NewAdminService(deps), admin.ID, "hyrox", "Hyrox"); err != nil {
		t.Fatal(err)
	}
	goals := service.NewGoalService(deps)
	start, end := int64(1_000_000), int64(9_000_000)
	g, err := goals.CreateGoal(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateGoalRequest{
		Name: "Two hyrox", PeriodStart: ts(start), PeriodEnd: ts(end),
		Sport: &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_SportType{SportType: "hyrox"}},
		Clauses: []*fitterv1.GoalClause{{
			Metric: fitterv1.ActivityMetric_ACTIVITY_METRIC_ACTIVITY_COUNT, Scope: fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
			Comparator: fitterv1.Comparator_COMPARATOR_GTE, Value: 2,
		}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	seedActivity(t, deps.Queries, 1, alice.ID, "hyrox", start+10, 0)
	seedActivity(t, deps.Queries, 2, alice.ID, "Hyrox", start+20, 0) // different case: not the same sport
	seedActivity(t, deps.Queries, 3, alice.ID, "Run", start+30, 5000)
	list, err := goals.ListGoals(ctxAs(alice.ID), connect.NewRequest(&fitterv1.ListGoalsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range list.Msg.GetGoals() {
		if x.GetId() == g.Msg.GetGoal().GetId() {
			if got := x.GetProgress().GetClauses()[0].GetCurrentValue(); got != 1 {
				t.Fatalf("custom-sport goal counted %v activities, want exactly the 1 with sport_type %q", got, "hyrox")
			}
			return
		}
	}
	t.Fatal("goal not listed")
}

// An empty exact-type selector would match every activity yet be filed under "Other".
func TestCreateGoal_EmptySportTypeSelectorIsRejected(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	_, err := service.NewGoalService(deps).CreateGoal(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateGoalRequest{
		Name: "x", PeriodStart: ts(1), PeriodEnd: ts(2),
		Sport: &fitterv1.SportSelector{Selector: &fitterv1.SportSelector_SportType{SportType: ""}},
		Clauses: []*fitterv1.GoalClause{{
			Metric: fitterv1.ActivityMetric_ACTIVITY_METRIC_ACTIVITY_COUNT, Scope: fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
			Comparator: fitterv1.Comparator_COMPARATOR_GTE, Value: 1,
		}},
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty sport_type selector: got %v, want InvalidArgument", err)
	}
}
