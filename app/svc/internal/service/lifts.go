package service

import (
	"context"
	"database/sql"
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

// attachLifts fills Activity.Lifts (heaviest kg per exercise key) for the manual activities
// in acts: from their stations, or from activity_lifts for an activity a previous-release
// container wrote without blocks. Strava rows never carry lifts.
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
	stations, err := q.ListStationsForActivities(ctx, ids)
	if err != nil {
		return err
	}
	legacy, err := q.ListLiftsForActivities(ctx, ids)
	if err != nil {
		return err
	}
	byID := map[int64]map[string]float64{}
	withBlocks := map[int64]bool{}
	add := func(activityID int64, exercise string, kg float64) {
		m := byID[activityID]
		if m == nil {
			m = map[string]float64{}
			byID[activityID] = m
		}
		if kg > m[exercise] {
			m[exercise] = kg
		}
	}
	for _, r := range stations {
		withBlocks[r.ActivityID] = true
		if r.WeightKg.Valid {
			add(r.ActivityID, r.ExerciseKey, r.WeightKg.Float64)
		}
	}
	for _, r := range legacy {
		if !withBlocks[r.ActivityID] {
			add(r.ActivityID, r.Exercise, r.WeightKg)
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
