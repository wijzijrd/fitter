package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

const maxExerciseNameLen = 60

// The DB stores muscles and measures as the text keys its CHECK constraints list; these
// tables are the one place they meet the proto enums.
var muscleKeys = map[fitterv1.Muscle]string{
	fitterv1.Muscle_MUSCLE_CHEST:      "chest",
	fitterv1.Muscle_MUSCLE_BACK:       "back",
	fitterv1.Muscle_MUSCLE_SHOULDERS:  "shoulders",
	fitterv1.Muscle_MUSCLE_BICEPS:     "biceps",
	fitterv1.Muscle_MUSCLE_TRICEPS:    "triceps",
	fitterv1.Muscle_MUSCLE_FOREARMS:   "forearms",
	fitterv1.Muscle_MUSCLE_CORE:       "core",
	fitterv1.Muscle_MUSCLE_GLUTES:     "glutes",
	fitterv1.Muscle_MUSCLE_QUADS:      "quads",
	fitterv1.Muscle_MUSCLE_HAMSTRINGS: "hamstrings",
	fitterv1.Muscle_MUSCLE_CALVES:     "calves",
	fitterv1.Muscle_MUSCLE_FULL_BODY:  "full_body",
	fitterv1.Muscle_MUSCLE_CARDIO:     "cardio",
}

var measureKeys = map[fitterv1.ExerciseMeasure]string{
	fitterv1.ExerciseMeasure_EXERCISE_MEASURE_WEIGHT_REPS: "weight_reps",
	fitterv1.ExerciseMeasure_EXERCISE_MEASURE_REPS:        "reps",
	fitterv1.ExerciseMeasure_EXERCISE_MEASURE_DISTANCE:    "distance",
	fitterv1.ExerciseMeasure_EXERCISE_MEASURE_TIME:        "time",
}

func keyToMuscle(k string) fitterv1.Muscle {
	for m, key := range muscleKeys {
		if key == k {
			return m
		}
	}
	return fitterv1.Muscle_MUSCLE_UNSPECIFIED
}

func keyToMeasure(k string) fitterv1.ExerciseMeasure {
	for m, key := range measureKeys {
		if key == k {
			return m
		}
	}
	return fitterv1.ExerciseMeasure_EXERCISE_MEASURE_UNSPECIFIED
}

func exerciseToProto(e db.Exercise, useCount int32) *fitterv1.Exercise {
	out := &fitterv1.Exercise{
		Id: e.ID, Name: e.Name, Key: e.Key, Measure: keyToMeasure(e.Measure), Loaded: e.Loaded, UseCount: useCount,
		Archived: e.ArchivedAt.Valid,
	}
	for _, m := range e.Muscles {
		out.Muscles = append(out.Muscles, keyToMuscle(m))
	}
	return out
}

// exerciseFields is a validated exercise as the create and update RPCs share it.
type exerciseFields struct {
	name, key, measure string
	loaded             bool
	muscles            []string
}

// validateExerciseFields trims and collapses the name (case is kept for display; the key
// folds it), and checks measure, loaded and the muscle list against the enums.
func validateExerciseFields(name string, measure fitterv1.ExerciseMeasure, loaded bool, muscles []fitterv1.Muscle) (exerciseFields, error) {
	display := strings.Join(strings.Fields(name), " ")
	key := normalizeExercise(display)
	if key == "" || len(display) > maxExerciseNameLen {
		return exerciseFields{}, invalid("an exercise name is 1 to 60 characters")
	}
	mk, ok := measureKeys[measure]
	if !ok {
		return exerciseFields{}, invalid("pick how the exercise is measured")
	}
	if loaded && mk != "distance" && mk != "time" {
		return exerciseFields{}, invalid("only distance and time exercises can take an optional weight")
	}
	if len(muscles) == 0 {
		return exerciseFields{}, invalid("pick at least one muscle")
	}
	seen := map[string]bool{}
	keys := make([]string, 0, len(muscles))
	for _, m := range muscles {
		k, ok := muscleKeys[m]
		if !ok {
			return exerciseFields{}, invalid("unknown muscle")
		}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return exerciseFields{name: display, key: key, measure: mk, loaded: loaded, muscles: keys}, nil
}

func (s *ExerciseService) ListExercises(ctx context.Context, _ *connect.Request[fitterv1.ListExercisesRequest]) (*connect.Response[fitterv1.ListExercisesResponse], error) {
	rows, err := s.Queries.ListExercises(ctx)
	if err != nil {
		return nil, internalErr(err)
	}
	out := make([]*fitterv1.Exercise, len(rows))
	for i, r := range rows {
		out[i] = exerciseToProto(db.Exercise{
			ID: r.ID, Name: r.Name, Key: r.Key, Measure: r.Measure, Loaded: r.Loaded, Muscles: r.Muscles,
			CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, ArchivedAt: r.ArchivedAt,
		}, r.UseCount)
	}
	return connect.NewResponse(&fitterv1.ListExercisesResponse{Exercises: out}), nil
}

// CreateExercise adds to the shared library; it is usable straight away. A name that
// already exists (same key) returns the stored exercise instead of an error, so the form
// can just select it. If that exercise was archived, adding it again restores it: the key is
// unique, so there is no second copy to create, and its history is the right one to log against.
func (s *ExerciseService) CreateExercise(ctx context.Context, req *connect.Request[fitterv1.CreateExerciseRequest]) (*connect.Response[fitterv1.CreateExerciseResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	m := req.Msg
	f, err := validateExerciseFields(m.GetName(), m.GetMeasure(), m.GetLoaded(), m.GetMuscles())
	if err != nil {
		return nil, err
	}
	existing := func() (*connect.Response[fitterv1.CreateExerciseResponse], error) {
		e, err := s.Queries.GetExerciseByKey(ctx, f.key)
		if err != nil {
			return nil, err
		}
		if e.ArchivedAt.Valid {
			if e, err = s.Queries.SetExerciseArchived(ctx, db.SetExerciseArchivedParams{ID: e.ID}); err != nil {
				return nil, err
			}
			if err := audit(ctx, s.Queries, userID, "restore_exercise", e.ID, e.Key); err != nil {
				return nil, err
			}
		}
		n, err := s.Queries.CountExerciseUse(ctx, e.ID)
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(&fitterv1.CreateExerciseResponse{Exercise: exerciseToProto(e, n), AlreadyExisted: true}), nil
	}
	if resp, err := existing(); err == nil {
		return resp, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, internalErr(err)
	}
	row, err := s.Queries.CreateExercise(ctx, db.CreateExerciseParams{
		ID: uuid.NewString(), Name: f.name, Key: f.key, Measure: f.measure, Loaded: f.loaded, Muscles: f.muscles,
		CreatedBy: sql.NullString{String: userID, Valid: true}, CreatedAt: time.Now().Unix(),
	})
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" { // lost a race with another add of the same name
		if resp, err := existing(); err == nil {
			return resp, nil
		}
	}
	if err != nil {
		return nil, internalErr(err)
	}
	return connect.NewResponse(&fitterv1.CreateExerciseResponse{Exercise: exerciseToProto(row, 0)}), nil
}
