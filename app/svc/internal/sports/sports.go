// Package sports defines the curated sport-grouping taxonomy used by goals
// to select which activities count toward them. It has zero dependencies on
// the rest of the service, so it is trivially unit-testable.
package sports

// Group is a curated bucket of related Strava sport_types.
type Group string

const (
	GroupRun      Group = "run"
	GroupRide     Group = "ride"
	GroupSwim     Group = "swim"
	GroupWalk     Group = "walk"
	GroupHike     Group = "hike"
	GroupWinter   Group = "winter"
	GroupWater    Group = "water"
	GroupStrength Group = "strength"
	GroupOther    Group = "other"
)

// GroupDef describes one curated group: its key, display label, and the
// Strava sport_type strings that belong to it.
type GroupDef struct {
	Group      Group
	Label      string
	SportTypes []string
}

// groups is the single source of truth for the sport taxonomy. Order is
// user-visible (metadata RPC, UI dropdown) and must stay stable.
var groups = []GroupDef{
	{
		Group:      GroupRun,
		Label:      "Run",
		SportTypes: []string{"Run", "TrailRun", "VirtualRun", "TrackRun"},
	},
	{
		Group: GroupRide,
		Label: "Ride",
		SportTypes: []string{
			"Ride", "GravelRide", "MountainBikeRide", "EBikeRide",
			"EMountainBikeRide", "VirtualRide", "Handcycle", "Velomobile",
		},
	},
	{
		Group:      GroupSwim,
		Label:      "Swim",
		SportTypes: []string{"Swim"},
	},
	{
		Group:      GroupWalk,
		Label:      "Walk",
		SportTypes: []string{"Walk"},
	},
	{
		Group:      GroupHike,
		Label:      "Hike",
		SportTypes: []string{"Hike", "Snowshoe"},
	},
	{
		Group:      GroupWinter,
		Label:      "Winter",
		SportTypes: []string{"AlpineSki", "BackcountrySki", "NordicSki", "Snowboard", "IceSkate"},
	},
	{
		Group: GroupWater,
		Label: "Water",
		SportTypes: []string{
			"Rowing", "VirtualRow", "Canoeing", "Kayaking", "StandUpPaddling",
			"Surfing", "Kitesurf", "Windsurf", "Sail",
		},
	},
	{
		Group: GroupStrength,
		Label: "Strength",
		SportTypes: []string{
			"WeightTraining", "Workout", "Crossfit", "HighIntensityIntervalTraining",
			"Pilates", "Yoga", "Elliptical", "StairStepper",
		},
	},
	{
		Group:      GroupOther,
		Label:      "Other",
		SportTypes: nil, // fallback: not matched by SportTypes membership, but by GroupOf's default
	},
}

// sportTypeToGroup is built once from groups for O(1) lookups in GroupOf.
var sportTypeToGroup = func() map[string]Group {
	m := make(map[string]Group)
	for _, g := range groups {
		for _, st := range g.SportTypes {
			m[st] = g.Group
		}
	}
	return m
}()

// labelByGroup is built once from groups for O(1) lookups in LabelFor.
var labelByGroup = func() map[Group]string {
	m := make(map[Group]string, len(groups))
	for _, g := range groups {
		m[g.Group] = g.Label
	}
	return m
}()

// Groups returns the curated group definitions in stable, user-visible
// order. Callers must not mutate the returned slice or its SportTypes.
func Groups() []GroupDef {
	out := make([]GroupDef, len(groups))
	copy(out, groups)
	return out
}

// GroupOf maps a Strava sport_type (and, as a fallback, the legacy type) to
// a curated Group. It never errors: an unmapped sport_type resolves to
// GroupOther. When sportType is empty (seeded rows or pre-resync rows may
// only have the legacy type field), activityType is used instead.
func GroupOf(sportType, activityType string) Group {
	key := sportType
	if key == "" {
		key = activityType
	}
	if g, ok := sportTypeToGroup[key]; ok {
		return g
	}
	return GroupOther
}

// Selector picks which activities a goal counts. The zero value (both
// fields empty) matches every sport. Group is set for a group-scoped
// selector; SportType is set for an exact-type selector. The two are
// mutually exclusive.
type Selector struct {
	Group     Group  // set for a group-scoped selector
	SportType string // set for an exact-type selector; mutually exclusive with Group
}

// Matches reports whether an activity (identified by its sport_type and
// legacy type) satisfies sel. A zero-value Selector matches everything.
func Matches(sel Selector, sportType, activityType string) bool {
	if sel.Group == "" && sel.SportType == "" {
		return true
	}
	if sel.SportType != "" {
		key := sportType
		if key == "" {
			key = activityType
		}
		return key == sel.SportType
	}
	return GroupOf(sportType, activityType) == sel.Group
}

// LabelFor returns the display label for a Group, or "" if g is not a known
// group.
func LabelFor(g Group) string {
	return labelByGroup[g]
}

// ValidGroup reports whether s names a known Group.
func ValidGroup(s string) bool {
	_, ok := labelByGroup[Group(s)]
	return ok
}
