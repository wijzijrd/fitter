package service

import (
	"testing"

	"github.com/fitter/svc/internal/db"
)

func TestGoalKindOf(t *testing.T) {
	agg, single := db.GoalClause{Scope: "aggregate"}, db.GoalClause{Scope: "single"}
	cases := []struct {
		name       string
		recurrence string
		clauses    []db.GoalClause
		want       string
	}{
		{"totals add up", "", []db.GoalClause{agg}, kindTotal},
		{"several totals", "none", []db.GoalClause{agg, agg}, kindTotal},
		{"no clauses at all", "", nil, kindTotal},
		{"a single-activity clause makes it a performance goal", "", []db.GoalClause{single}, kindPerformance},
		{"a mix of total and single is a performance goal", "", []db.GoalClause{agg, single}, kindPerformance},
		{"daily repeats", "daily", []db.GoalClause{agg}, kindHabit},
		{"weekly repeats", "weekly", []db.GoalClause{agg}, kindHabit},
		{"monthly repeats", "monthly", []db.GoalClause{agg}, kindHabit},
		{"repeating wins over a single clause", "weekly", []db.GoalClause{single}, kindHabit},
		{"an unknown recurrence value is not a habit", "fortnightly", []db.GoalClause{agg}, kindTotal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := goalKindOf(db.Goal{Recurrence: c.recurrence}, c.clauses); got != c.want {
				t.Fatalf("goalKindOf = %q, want %q", got, c.want)
			}
		})
	}
}

func TestKindOrderAndLabels(t *testing.T) {
	if !(kindOrder[kindTotal] < kindOrder[kindPerformance] && kindOrder[kindPerformance] < kindOrder[kindHabit]) {
		t.Fatalf("kind order = %v, want total, then single efforts, then habits", kindOrder)
	}
	if kindLabel[kindTotal] != "" || kindLabel[kindHabit] != "Habits" || kindLabel[kindPerformance] != "Single efforts" {
		t.Fatalf("kind labels = %v: totals carry no suffix so the common case stays uncluttered", kindLabel)
	}
}
