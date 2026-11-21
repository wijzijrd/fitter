package service

import (
	"time"

	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/progress"
)

// leaderboardWindow resolves a request's window to a UTC calendar week
// (Monday start) or month containing anchor. nil = score the whole campaign.
//
// shortcut: boundaries are UTC, not the viewer's timezone, so a Sunday-night
// run can land in the next UTC week; take a utc offset on the request if
// anyone notices.
func leaderboardWindow(w fitterv1.LeaderboardWindow, anchor time.Time) *progress.Window {
	a := anchor.UTC()
	y, m, d := a.Date()
	switch w {
	case fitterv1.LeaderboardWindow_LEADERBOARD_WINDOW_WEEK:
		start := time.Date(y, m, d-(int(a.Weekday())+6)%7, 0, 0, 0, 0, time.UTC)
		return &progress.Window{Start: start.Unix(), End: start.AddDate(0, 0, 7).Unix() - 1}
	case fitterv1.LeaderboardWindow_LEADERBOARD_WINDOW_MONTH:
		start := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
		return &progress.Window{Start: start.Unix(), End: start.AddDate(0, 1, 0).Unix() - 1}
	default:
		return nil
	}
}

// windowedClauseDefs is clauseDefsToProto, except that on a windowed board
// the clause targets are the pro-rated ones actually scored against, so the
// UI's "x to catch" amounts stay consistent with the ratios.
func windowedClauseDefs(clauses []db.GoalClause, used progress.Goal, windowed bool) []*fitterv1.GoalClause {
	out := clauseDefsToProto(clauses)
	if windowed && len(used.Clauses) == len(out) {
		for i := range out {
			out[i].Value = used.Clauses[i].Value
		}
	}
	return out
}
