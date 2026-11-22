package service

import (
	"database/sql"

	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/progress"
	"github.com/fitter/svc/internal/sports"
)

// Goal kinds. A leaderboard ranks goals of one kind together: a total's ratio is a share of the
// whole target, a habit's is buckets hit over buckets so far, and a performance goal's is how
// close one activity got, so ranking across kinds compares different things.
const (
	kindTotal       = "total"
	kindPerformance = "performance"
	kindHabit       = "habit"
)

// leaderboardGroupKey is what a leaderboard group is: a sport group and a goal kind.
type leaderboardGroupKey struct{ sport, kind string }

// kindOrder is the display order within a sport group.
var kindOrder = map[string]int{kindTotal: 0, kindPerformance: 1, kindHabit: 2}

var kindLabel = map[string]string{kindPerformance: "Single efforts", kindHabit: "Habits"}

// goalKindOf classifies a goal: repeating goals are habits, goals with any single-activity clause
// are performance goals, everything else adds up over the period.
func goalKindOf(g db.Goal, clauses []db.GoalClause) string {
	if recurrenceFromDB(g.Recurrence) != progress.RecurNone {
		return kindHabit
	}
	for _, c := range clauses {
		if scopeFromDB(c.Scope) == fitterv1.ClauseScope_CLAUSE_SCOPE_SINGLE {
			return kindPerformance
		}
	}
	return kindTotal
}

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
