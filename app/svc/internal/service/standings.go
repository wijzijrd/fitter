package service

import (
	"sort"

	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/progress"
)

const earlyFinishLead = 24 * 60 * 60 // seconds before period end

// rankPeople collapses per-goal results into one standing per user: average
// goal ratio, then goals satisfied, then name.
func rankPeople(goals []db.Goal, results map[string]progress.Result, names, avatars map[string]string) []*fitterv1.PersonStanding {
	byUser := map[string]*fitterv1.PersonStanding{}
	sums := map[string]float64{}
	var order []string
	for _, g := range goals {
		p, ok := byUser[g.UserID]
		if !ok {
			p = &fitterv1.PersonStanding{UserId: g.UserID, UserName: names[g.ID], UserAvatarUrl: avatars[g.ID]}
			byUser[g.UserID] = p
			order = append(order, g.UserID)
		}
		res := results[g.ID]
		p.GoalCount++
		sums[g.UserID] += res.Ratio
		if res.Satisfied {
			p.GoalsSatisfied++
			if res.SatisfiedAt != 0 && res.SatisfiedAt < g.PeriodEnd-earlyFinishLead {
				p.EarlyFinishes++
			}
		}
	}
	out := make([]*fitterv1.PersonStanding, 0, len(order))
	for _, id := range order {
		p := byUser[id]
		p.AverageRatio = sums[id] / float64(p.GoalCount)
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.AverageRatio != b.AverageRatio {
			return a.AverageRatio > b.AverageRatio
		}
		if a.GoalsSatisfied != b.GoalsSatisfied {
			return a.GoalsSatisfied > b.GoalsSatisfied
		}
		if a.UserName != b.UserName {
			return a.UserName < b.UserName
		}
		return a.UserId < b.UserId
	})
	for i, p := range out {
		p.Rank = int32(i + 1)
	}
	return out
}
