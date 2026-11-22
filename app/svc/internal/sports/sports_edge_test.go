package sports_test

import (
	"strings"
	"testing"

	"github.com/fitter/svc/internal/sports"
)

// Every curated type must resolve back to the group that lists it, for all groups at once.
// Catches a type added to a group but shadowed by a later group's entry, or a typo in the map build.
func TestEveryCuratedTypeResolvesToItsOwnGroup(t *testing.T) {
	n := 0
	for _, g := range sports.Groups() {
		for _, st := range g.SportTypes {
			n++
			if got := sports.GroupOf(st, ""); got != g.Group {
				t.Errorf("GroupOf(%q) = %q, want %q", st, got, g.Group)
			}
			if got := sports.GroupOf("", st); got != g.Group {
				t.Errorf("GroupOf(\"\", legacy %q) = %q, want %q", st, got, g.Group)
			}
		}
	}
	if n < 30 {
		t.Fatalf("only %d curated sport types; the table looks truncated", n)
	}
}

func TestGroupKeysAndLabelsAreUniqueAndNonEmpty(t *testing.T) {
	keys, labels := map[sports.Group]bool{}, map[string]bool{}
	for _, g := range sports.Groups() {
		if g.Group == "" || strings.TrimSpace(g.Label) == "" {
			t.Errorf("group %+v has an empty key or label", g)
		}
		if keys[g.Group] {
			t.Errorf("duplicate group key %q", g.Group)
		}
		if labels[g.Label] {
			t.Errorf("duplicate group label %q", g.Label)
		}
		keys[g.Group], labels[g.Label] = true, true
		// Group keys are used verbatim as custom-metric sport refs and URL-ish values.
		if g.Group != sports.Group(strings.ToLower(string(g.Group))) || strings.ContainsAny(string(g.Group), " &/") {
			t.Errorf("group key %q should be lower-case with no spaces or symbols", g.Group)
		}
	}
}

// Only the fallback group may be empty, and it must be last so "Other" sorts after real sports.
func TestOnlyOtherHasNoTypesAndItIsLast(t *testing.T) {
	gs := sports.Groups()
	for i, g := range gs {
		empty := len(g.SportTypes) == 0
		if empty != (g.Group == sports.GroupOther) {
			t.Errorf("group %q: empty=%v, but only %q may be empty", g.Group, empty, sports.GroupOther)
		}
		if g.Group == sports.GroupOther && i != len(gs)-1 {
			t.Errorf("%q is at position %d, want last", g.Group, i)
		}
	}
}

// Groups() hands out a copy so a caller can't corrupt the taxonomy. The copy must be deep:
// the SportTypes slices are shared backing arrays unless copied too.
func TestGroupsCopyIsolatesSportTypes(t *testing.T) {
	first := sports.Groups()
	orig := first[0].SportTypes[0]
	first[0].SportTypes[0] = "Corrupted"
	defer func() { first[0].SportTypes[0] = orig }() // don't poison other tests if this fails

	if got := sports.Groups()[0].SportTypes[0]; got != orig {
		t.Fatalf("mutating a returned SportTypes slice changed the package table: now %q, was %q", got, orig)
	}
	if got := sports.GroupOf(orig, ""); got != sports.GroupRun {
		t.Fatalf("GroupOf(%q) = %q after a caller mutated its copy", orig, got)
	}
}

func TestGroupOfEdgeInputs(t *testing.T) {
	cases := []struct {
		name, sportType, legacy string
		want                    sports.Group
	}{
		{"both empty", "", "", sports.GroupOther},
		{"lower-case is a different type", "run", "", sports.GroupOther},
		{"upper-case is a different type", "RUN", "", sports.GroupOther},
		{"leading space is not trimmed", " Run", "", sports.GroupOther},
		{"trailing space is not trimmed", "Run ", "", sports.GroupOther},
		{"unicode lookalike", "Rυn", "", sports.GroupOther},
		{"huge string", strings.Repeat("x", 100_000), "", sports.GroupOther},
		{"legacy used only when sport_type is empty", "", "Ride", sports.GroupRide},
		// An unknown sport_type does NOT fall back to the legacy type: Strava's newer sport_type is
		// authoritative even when we haven't curated it yet. Pinned so a change is deliberate.
		{"unknown sport_type beats a known legacy type", "PickleBall", "Run", sports.GroupOther},
		{"known sport_type beats a different legacy type", "Swim", "Run", sports.GroupSwim},
		{"a custom sport's key is not a curated type", "powerlifting", "", sports.GroupOther},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sports.GroupOf(c.sportType, c.legacy); got != c.want {
				t.Fatalf("GroupOf(%q, %q) = %q, want %q", c.sportType, c.legacy, got, c.want)
			}
		})
	}
}

// One representative type per group, matched against a selector for every group: each type
// matches exactly its own group's selector and no other.
func TestMatchesEveryGroupAgainstEveryGroup(t *testing.T) {
	for _, owner := range sports.Groups() {
		if len(owner.SportTypes) == 0 {
			continue
		}
		rep := owner.SportTypes[0]
		for _, sel := range sports.Groups() {
			want := owner.Group == sel.Group
			if got := sports.Matches(sports.Selector{Group: sel.Group}, rep, rep); got != want {
				t.Errorf("Matches(group %q, %q) = %v, want %v", sel.Group, rep, got, want)
			}
		}
	}
}

func TestMatchesEdgeSelectors(t *testing.T) {
	cases := []struct {
		name string
		sel  sports.Selector
		st   string
		leg  string
		want bool
	}{
		{"zero selector matches an empty activity", sports.Selector{}, "", "", true},
		{"zero selector matches an unknown type", sports.Selector{}, "Zorb", "", true},
		{"unknown group matches nothing", sports.Selector{Group: "nonsense"}, "Run", "", false},
		{"other group matches an unmapped type", sports.Selector{Group: sports.GroupOther}, "Zorb", "", true},
		{"other group matches an activity with no type at all", sports.Selector{Group: sports.GroupOther}, "", "", true},
		{"other group does not match a curated type", sports.Selector{Group: sports.GroupOther}, "Run", "", false},
		{"exact type is case-sensitive", sports.Selector{SportType: "run"}, "Run", "", false},
		{"exact type matches itself", sports.Selector{SportType: "TrailRun"}, "TrailRun", "", true},
		{"exact type does not match its group-mates", sports.Selector{SportType: "TrailRun"}, "Run", "", false},
		{"exact type falls back to the legacy type when sport_type is empty", sports.Selector{SportType: "Run"}, "", "Run", true},
		{"exact type: an unknown sport_type is not rescued by the legacy type", sports.Selector{SportType: "Run"}, "Zorb", "Run", false},
		// SportType wins when both are set (the proto oneof makes this unreachable over the wire).
		{"both set: sport type wins over group", sports.Selector{Group: sports.GroupRide, SportType: "Run"}, "Run", "", true},
		{"both set: group is ignored", sports.Selector{Group: sports.GroupRun, SportType: "Ride"}, "Run", "", false},
		{"a custom sport's key selects activities of that exact type", sports.Selector{SportType: "powerlifting"}, "powerlifting", "", true},
		{"a custom sport's key selects nothing else", sports.Selector{SportType: "powerlifting"}, "WeightTraining", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sports.Matches(c.sel, c.st, c.leg); got != c.want {
				t.Fatalf("Matches(%+v, %q, %q) = %v, want %v", c.sel, c.st, c.leg, got, c.want)
			}
		})
	}
}

func TestLabelForAndValidGroupAgreeForEveryGroup(t *testing.T) {
	for _, g := range sports.Groups() {
		if !sports.ValidGroup(string(g.Group)) || sports.LabelFor(g.Group) != g.Label {
			t.Errorf("group %q: ValidGroup=%v LabelFor=%q, want valid with label %q", g.Group, sports.ValidGroup(string(g.Group)), sports.LabelFor(g.Group), g.Label)
		}
	}
	for _, bad := range []string{"", " ", "Run", "RUN", "run ", "circuits ", "hiit & circuits", "__together__"} {
		if sports.ValidGroup(bad) {
			t.Errorf("ValidGroup(%q) = true, want false (keys are exact and lower-case)", bad)
		}
		if got := sports.LabelFor(sports.Group(bad)); got != "" {
			t.Errorf("LabelFor(%q) = %q, want empty", bad, got)
		}
	}
}

// Strava sport types we deliberately don't curate: they land in "Other". Listing them keeps
// that a decision rather than an accident, and flags it when one is later given a group.
func TestKnownUncuratedStravaTypesFallToOther(t *testing.T) {
	for _, st := range []string{
		"Badminton", "Golf", "Soccer", "Tennis", "RockClimbing", "InlineSkate", "RollerSki",
		"Skateboard", "Wheelchair", "Pickleball", "Racquetball", "Squash", "TableTennis", "Velomobile ",
	} {
		if got := sports.GroupOf(st, st); got != sports.GroupOther {
			t.Logf("%q is now curated into %q; update this list if that was deliberate", st, got)
		}
	}
}
