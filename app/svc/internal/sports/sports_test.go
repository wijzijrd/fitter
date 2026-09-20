package sports_test

import (
	"reflect"
	"testing"

	"github.com/fitter/svc/internal/sports"
)

func TestNoSportTypeInTwoGroups(t *testing.T) {
	seen := make(map[string]sports.Group)
	for _, g := range sports.Groups() {
		for _, st := range g.SportTypes {
			if prior, ok := seen[st]; ok {
				t.Fatalf("sport_type %q appears in both group %q and group %q", st, prior, g.Group)
			}
			seen[st] = g.Group
		}
	}
	if len(seen) == 0 {
		t.Fatal("expected at least one sport_type across all groups")
	}
}

func TestGroupsOrderStable(t *testing.T) {
	first := sports.Groups()
	second := sports.Groups()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Groups() order/content not stable across calls:\n%+v\nvs\n%+v", first, second)
	}

	var firstOrder []sports.Group
	for _, g := range first {
		firstOrder = append(firstOrder, g.Group)
	}
	wantOrder := []sports.Group{
		sports.GroupRun, sports.GroupRide, sports.GroupSwim, sports.GroupWalk,
		sports.GroupHike, sports.GroupWinter, sports.GroupWater, sports.GroupStrength,
		sports.GroupOther,
	}
	if !reflect.DeepEqual(firstOrder, wantOrder) {
		t.Fatalf("Groups() order = %v, want %v", firstOrder, wantOrder)
	}
}

func TestGroupsReturnsCopy(t *testing.T) {
	first := sports.Groups()
	first[0].Label = "mutated"
	second := sports.Groups()
	if second[0].Label == "mutated" {
		t.Fatal("Groups() returned a slice that aliases internal state")
	}
}

func TestGroupOfUnmappedFallsBackToOther(t *testing.T) {
	if g := sports.GroupOf("SomeUnknownFutureSport", ""); g != sports.GroupOther {
		t.Fatalf("GroupOf(unmapped) = %q, want %q", g, sports.GroupOther)
	}
}

func TestGroupOfLegacyTypeFallback(t *testing.T) {
	// Empty sport_type, legacy `type` only — must resolve via the
	// activityType fallback path.
	if g := sports.GroupOf("", "Run"); g != sports.GroupRun {
		t.Fatalf(`GroupOf("", "Run") = %q, want %q`, g, sports.GroupRun)
	}
}

func TestGroupOfPrefersSportType(t *testing.T) {
	if g := sports.GroupOf("Swim", "Run"); g != sports.GroupSwim {
		t.Fatalf(`GroupOf("Swim", "Run") = %q, want %q (sport_type should take priority)`, g, sports.GroupSwim)
	}
}

func TestMatchesZeroValueSelectorMatchesAnything(t *testing.T) {
	var sel sports.Selector
	if !sports.Matches(sel, "Run", "Run") {
		t.Fatal("zero-value Selector should match Run")
	}
	if !sports.Matches(sel, "TotallyUnknownSport", "") {
		t.Fatal("zero-value Selector should match any unmapped sport")
	}
	if !sports.Matches(sel, "", "") {
		t.Fatal("zero-value Selector should match even empty sport/type")
	}
}

func TestMatchesGroupSelector(t *testing.T) {
	sel := sports.Selector{Group: sports.GroupRun}
	for _, st := range []string{"Run", "TrailRun", "VirtualRun", "TrackRun"} {
		if !sports.Matches(sel, st, "") {
			t.Errorf("group selector %q should match sport_type %q", sel.Group, st)
		}
	}
	for _, st := range []string{"Ride", "Swim", "Walk"} {
		if sports.Matches(sel, st, "") {
			t.Errorf("group selector %q should reject sport_type %q", sel.Group, st)
		}
	}
}

func TestMatchesExactSelector(t *testing.T) {
	sel := sports.Selector{SportType: "TrailRun"}
	if !sports.Matches(sel, "TrailRun", "Run") {
		t.Fatal("exact selector should match its literal sport_type")
	}
	if sports.Matches(sel, "Run", "Run") {
		t.Fatal("exact selector should reject a different sport_type, even in the same group")
	}
	if sports.Matches(sel, "VirtualRun", "") {
		t.Fatal("exact selector should reject sibling sport_types in the same group")
	}
}

func TestLabelFor(t *testing.T) {
	if got := sports.LabelFor(sports.GroupRun); got != "Run" {
		t.Fatalf("LabelFor(GroupRun) = %q, want %q", got, "Run")
	}
	if got := sports.LabelFor(sports.Group("nonexistent")); got != "" {
		t.Fatalf("LabelFor(unknown) = %q, want empty string", got)
	}
}

func TestValidGroup(t *testing.T) {
	if !sports.ValidGroup("run") {
		t.Fatal(`ValidGroup("run") = false, want true`)
	}
	if sports.ValidGroup("nonexistent") {
		t.Fatal(`ValidGroup("nonexistent") = true, want false`)
	}
}
