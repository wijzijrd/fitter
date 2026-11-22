package service_test

import (
	"testing"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/service"
	"github.com/fitter/svc/internal/sports"
)

// The catalogue the UI builds its sport picker from must be exactly the curated taxonomy, in
// order, with nothing added by custom sports (those are reached through admin and goal selectors).
func TestListSports_IsTheCuratedTaxonomyInOrder(t *testing.T) {
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	if _, err := createSport(service.NewAdminService(deps), admin.ID, "hyrox", "Hyrox"); err != nil {
		t.Fatal(err)
	}
	resp, err := service.NewCatalogService(deps).ListSports(ctxAs(admin.ID), connect.NewRequest(&fitterv1.ListSportsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	want := sports.Groups()
	got := resp.Msg.GetGroups()
	if len(got) != len(want) {
		t.Fatalf("%d groups, want %d (a custom sport must not appear as a group)", len(got), len(want))
	}
	for i, g := range got {
		if g.GetGroup() != string(want[i].Group) || g.GetLabel() != want[i].Label || len(g.GetSportTypes()) != len(want[i].SportTypes) {
			t.Errorf("group %d = %s/%q/%d types, want %s/%q/%d", i, g.GetGroup(), g.GetLabel(), len(g.GetSportTypes()), want[i].Group, want[i].Label, len(want[i].SportTypes))
		}
	}
	if last := got[len(got)-1]; last.GetGroup() != "other" || len(last.GetSportTypes()) != 0 {
		t.Errorf("last group = %v, want other with no listed types", last)
	}
}
