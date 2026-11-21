package service

import (
	"database/sql"
	"context"
	"strings"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/metrics"
)

// normalizeExercise folds an exercise name to its lower-case key: trimmed,
// inner whitespace collapsed ("Bench  Press" -> "bench press").
func normalizeExercise(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// clauseDef narrows a max-weight Def to the clause's exercise; every other
// clause keeps the Def it was resolved to.
func clauseDef(def metrics.Def, c db.GoalClause) metrics.Def {
	if def.Key == metrics.MaxWeightKg && c.Exercise.Valid {
		return metrics.NewMaxWeightDef(c.Exercise.String)
	}
	return def
}

// attachLifts fills Activity.Lifts (heaviest kg per exercise) for the manual
// activities in acts, with one query. Strava rows never carry lifts.
func attachLifts(ctx context.Context, q *db.Queries, acts []metrics.Activity) error {
	var ids []int64
	for _, a := range acts {
		if a.StravaActivityID < 0 {
			ids = append(ids, a.StravaActivityID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.ListLiftsForActivities(ctx, ids)
	if err != nil {
		return err
	}
	byID := map[int64]map[string]float64{}
	for _, r := range rows {
		m := byID[r.ActivityID]
		if m == nil {
			m = map[string]float64{}
			byID[r.ActivityID] = m
		}
		if r.WeightKg > m[r.Exercise] {
			m[r.Exercise] = r.WeightKg
		}
	}
	for i := range acts {
		acts[i].Lifts = byID[acts[i].StravaActivityID]
	}
	return nil
}

func nullStringPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}
