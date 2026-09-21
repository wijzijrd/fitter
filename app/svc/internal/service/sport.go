package service

import (
	"database/sql"

	"github.com/fitter/svc/internal/sports"
)

// goalGroupKey resolves the sport-group bucket a goal belongs in for the
// leaderboard and timeline: its own curated sport_group if set; if instead
// it has an exact sport_type, that type's curated group (so e.g. a
// TrailRun goal lands in the same bucket as a plain Run goal); "" ("all
// sports") if the goal has neither.
func goalGroupKey(sportGroup, sportType sql.NullString) string {
	if sportGroup.Valid {
		return sportGroup.String
	}
	if sportType.Valid {
		return string(sports.GroupOf(sportType.String, ""))
	}
	return ""
}

// sportGroupOrder returns the fixed display order for sport groups (from
// sports.Groups()), with the "" ("all sports") group ordered last. Used to
// sort LeaderboardGroup/TimelineGroup slices deterministically.
func sportGroupOrder() map[string]int {
	defs := sports.Groups()
	order := make(map[string]int, len(defs)+1)
	for i, d := range defs {
		order[string(d.Group)] = i
	}
	order[""] = len(defs)
	return order
}

// sportGroupLabel returns the display label for a goalGroupKey value; ""
// ("all sports") gets a fixed label of its own since it isn't a curated
// sports.Group.
func sportGroupLabel(key string) string {
	if key == "" {
		return "All sports"
	}
	return sports.LabelFor(sports.Group(key))
}
