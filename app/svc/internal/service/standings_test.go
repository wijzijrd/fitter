package service

import (
	"fmt"
	"testing"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/progress"
)

// A has 5 goals averaging 60%, B has 3 averaging 70%: B must lead.
func TestRankPeople_AverageBeatsGoalCount(t *testing.T) {
	var goals []db.Goal
	results := map[string]progress.Result{}
	names, avatars := map[string]string{}, map[string]string{}
	add := func(user, name string, n int, ratio float64) {
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("%s%d", user, i)
			goals = append(goals, db.Goal{ID: id, UserID: user, PeriodEnd: 100 * 86400})
			results[id] = progress.Result{Ratio: ratio}
			names[id] = name
		}
	}
	add("a", "A", 5, 0.6)
	add("b", "B", 3, 0.7)
	got := rankPeople(goals, results, names, avatars)
	if len(got) != 2 || got[0].UserId != "b" || got[0].Rank != 1 || got[1].GoalCount != 5 {
		t.Fatalf("unexpected standings: %v", got)
	}
}

func TestRankPeople_EarlyFinish(t *testing.T) {
	end := int64(100 * 86400)
	goals := []db.Goal{{ID: "g1", UserID: "u", PeriodEnd: end}, {ID: "g2", UserID: "u", PeriodEnd: end}}
	results := map[string]progress.Result{
		"g1": {Ratio: 1, Satisfied: true, SatisfiedAt: end - 2*86400},
		"g2": {Ratio: 1, Satisfied: true, SatisfiedAt: end - 3600}, // final day
	}
	p := rankPeople(goals, results, map[string]string{}, map[string]string{})[0]
	if p.EarlyFinishes != 1 || p.GoalsSatisfied != 2 {
		t.Fatalf("early=%d satisfied=%d", p.EarlyFinishes, p.GoalsSatisfied)
	}
}
