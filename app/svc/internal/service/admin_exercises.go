package service

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
)

// rekeyExercise points everything that refers to an exercise by its key (goal clauses and
// the legacy lifts table) at newKey. Stations refer by id, so renames don't touch them.
func rekeyExercise(ctx context.Context, q *db.Queries, oldKey, newKey string) error {
	if oldKey == newKey {
		return nil
	}
	if err := q.RekeyGoalClauses(ctx, db.RekeyGoalClausesParams{Exercise: sql.NullString{String: oldKey, Valid: true}, Exercise_2: sql.NullString{String: newKey, Valid: true}}); err != nil {
		return err
	}
	return q.RekeyLegacyLifts(ctx, db.RekeyLegacyLiftsParams{Exercise: oldKey, Exercise_2: newKey})
}

func exerciseNotFound() error {
	return connect.NewError(connect.CodeNotFound, errors.New("exercise not found"))
}

// UpdateExercise edits name and muscles any time; measure and loaded only while no station
// uses the exercise, since existing results would stop matching them.
func (s *AdminService) UpdateExercise(ctx context.Context, req *connect.Request[fitterv1.UpdateExerciseRequest]) (*connect.Response[fitterv1.UpdateExerciseResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	actor, _ := auth.UserIDFromContext(ctx)
	m := req.Msg
	f, err := validateExerciseFields(m.GetName(), m.GetMeasure(), m.GetLoaded(), m.GetMuscles())
	if err != nil {
		return nil, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, internalErr(err)
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
	old, err := q.GetExercise(ctx, m.GetId())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, exerciseNotFound()
	} else if err != nil {
		return nil, internalErr(err)
	}
	use, err := q.CountExerciseUse(ctx, old.ID)
	if err != nil {
		return nil, internalErr(err)
	}
	if use > 0 && (f.measure != old.Measure || f.loaded != old.Loaded) {
		return nil, invalid("how an exercise is measured can't change while it is in use")
	}
	if f.key != old.Key {
		if _, err := q.GetExerciseByKey(ctx, f.key); err == nil {
			return nil, invalid("an exercise with that name already exists; merge them instead")
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, internalErr(err)
		}
	}
	row, err := q.UpdateExercise(ctx, db.UpdateExerciseParams{ID: old.ID, Name: f.name, Key: f.key, Measure: f.measure, Loaded: f.loaded, Muscles: f.muscles})
	if err != nil {
		return nil, internalErr(err)
	}
	if err := rekeyExercise(ctx, q, old.Key, row.Key); err != nil {
		return nil, internalErr(err)
	}
	if err := audit(ctx, q, actor, "update_exercise", row.ID, old.Key+" -> "+row.Key); err != nil {
		return nil, internalErr(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, internalErr(err)
	}
	return connect.NewResponse(&fitterv1.UpdateExerciseResponse{Exercise: exerciseToProto(row, use)}), nil
}

// DeleteExercise only removes an unused exercise; merging is the way to retire a used one.
func (s *AdminService) DeleteExercise(ctx context.Context, req *connect.Request[fitterv1.DeleteExerciseRequest]) (*connect.Response[fitterv1.DeleteExerciseResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	actor, _ := auth.UserIDFromContext(ctx)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, internalErr(err)
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
	e, err := q.GetExercise(ctx, req.Msg.GetId())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, exerciseNotFound()
	} else if err != nil {
		return nil, internalErr(err)
	}
	refs, err := q.CountExerciseRefs(ctx, db.CountExerciseRefsParams{ID: e.ID, Key: e.Key})
	if err != nil {
		return nil, internalErr(err)
	}
	if refs > 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("exercise is in use; merge it into another instead"))
	}
	if err := q.DeleteExercise(ctx, e.ID); err != nil {
		return nil, internalErr(err)
	}
	if err := audit(ctx, q, actor, "delete_exercise", e.ID, e.Key); err != nil {
		return nil, internalErr(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, internalErr(err)
	}
	return connect.NewResponse(&fitterv1.DeleteExerciseResponse{}), nil
}

// MergeExercises moves a duplicate's history and goal clauses onto the exercise to keep.
// The two must be measured the same way, and the target must take a weight if the source did.
func (s *AdminService) MergeExercises(ctx context.Context, req *connect.Request[fitterv1.MergeExercisesRequest]) (*connect.Response[fitterv1.MergeExercisesResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	actor, _ := auth.UserIDFromContext(ctx)
	srcID, dstID := req.Msg.GetSourceId(), req.Msg.GetTargetId()
	if srcID == dstID {
		return nil, invalid("pick a different exercise to merge into")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, internalErr(err)
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
	src, err := q.GetExercise(ctx, srcID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, exerciseNotFound()
	} else if err != nil {
		return nil, internalErr(err)
	}
	dst, err := q.GetExercise(ctx, dstID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, exerciseNotFound()
	} else if err != nil {
		return nil, internalErr(err)
	}
	if src.Measure != dst.Measure || (src.Loaded && !dst.Loaded) {
		return nil, invalid("those exercises are measured differently, so they can't be merged")
	}
	if err := q.RepointStations(ctx, db.RepointStationsParams{ExerciseID: src.ID, ExerciseID_2: dst.ID}); err != nil {
		return nil, internalErr(err)
	}
	if err := rekeyExercise(ctx, q, src.Key, dst.Key); err != nil {
		return nil, internalErr(err)
	}
	if err := q.DeleteExercise(ctx, src.ID); err != nil {
		return nil, internalErr(err)
	}
	if err := audit(ctx, q, actor, "merge_exercise", dst.ID, src.Key+" -> "+dst.Key); err != nil {
		return nil, internalErr(err)
	}
	use, err := q.CountExerciseUse(ctx, dst.ID)
	if err != nil {
		return nil, internalErr(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, internalErr(err)
	}
	return connect.NewResponse(&fitterv1.MergeExercisesResponse{Exercise: exerciseToProto(dst, use)}), nil
}
