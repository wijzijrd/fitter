package service_test

import (
	"context"
	"math"
	"testing"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/service"
)

func exerciseID(t *testing.T, deps *service.Deps, key string) string {
	t.Helper()
	e, err := deps.Queries.GetExerciseByKey(context.Background(), key)
	if err != nil {
		t.Fatalf("exercise %q: %v", key, err)
	}
	return e.ID
}

func newExercise(measure fitterv1.ExerciseMeasure, muscles ...fitterv1.Muscle) *fitterv1.CreateExerciseRequest {
	return &fitterv1.CreateExerciseRequest{Name: "Zercher squat", Measure: measure, Muscles: muscles}
}

func TestExercises_CreateDedupeAndList(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	svc := service.NewExerciseService(deps)
	ctx := ctxAs(alice.ID)

	resp, err := svc.CreateExercise(ctx, connect.NewRequest(newExercise(fitterv1.ExerciseMeasure_EXERCISE_MEASURE_WEIGHT_REPS, fitterv1.Muscle_MUSCLE_QUADS, fitterv1.Muscle_MUSCLE_QUADS, fitterv1.Muscle_MUSCLE_CORE)))
	if err != nil {
		t.Fatalf("CreateExercise: %v", err)
	}
	e := resp.Msg.GetExercise()
	if resp.Msg.GetAlreadyExisted() || e.GetKey() != "zercher squat" || len(e.GetMuscles()) != 2 {
		t.Fatalf("got %+v, want a new exercise with the duplicate muscle folded", resp.Msg)
	}

	again := newExercise(fitterv1.ExerciseMeasure_EXERCISE_MEASURE_REPS, fitterv1.Muscle_MUSCLE_BACK)
	again.Name = "  ZERCHER   squat "
	resp, err = svc.CreateExercise(ctx, connect.NewRequest(again))
	if err != nil || !resp.Msg.GetAlreadyExisted() || resp.Msg.GetExercise().GetId() != e.GetId() || resp.Msg.GetExercise().GetMeasure() != fitterv1.ExerciseMeasure_EXERCISE_MEASURE_WEIGHT_REPS {
		t.Fatalf("same key: resp=%+v err=%v, want the stored exercise flagged already_existed", resp.Msg, err)
	}

	list, err := svc.ListExercises(ctx, connect.NewRequest(&fitterv1.ListExercisesRequest{}))
	if err != nil {
		t.Fatalf("ListExercises: %v", err)
	}
	keys := map[string]bool{}
	for _, x := range list.Msg.GetExercises() {
		keys[x.GetKey()] = true
	}
	if !keys["zercher squat"] || !keys["bench press"] || !keys["sled push"] {
		t.Fatalf("list is missing the new exercise or seeded ones (%d total)", len(keys))
	}
}

func TestExercises_CreateValidation(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	svc := service.NewExerciseService(deps)
	for name, mod := range map[string]func(*fitterv1.CreateExerciseRequest){
		"empty name":     func(r *fitterv1.CreateExerciseRequest) { r.Name = "   " },
		"long name":      func(r *fitterv1.CreateExerciseRequest) { r.Name = string(make([]byte, 61)) + "x" },
		"no muscles":     func(r *fitterv1.CreateExerciseRequest) { r.Muscles = nil },
		"unknown muscle": func(r *fitterv1.CreateExerciseRequest) { r.Muscles = []fitterv1.Muscle{99} },
		"no measure": func(r *fitterv1.CreateExerciseRequest) {
			r.Measure = fitterv1.ExerciseMeasure_EXERCISE_MEASURE_UNSPECIFIED
		},
		"loaded on reps": func(r *fitterv1.CreateExerciseRequest) { r.Loaded = true },
	} {
		r := newExercise(fitterv1.ExerciseMeasure_EXERCISE_MEASURE_REPS, fitterv1.Muscle_MUSCLE_CORE)
		mod(r)
		if _, err := svc.CreateExercise(ctxAs(alice.ID), connect.NewRequest(r)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}
}

func stationOf(deps *service.Deps, t *testing.T, key string, mod func(*fitterv1.Station)) *fitterv1.Station {
	st := &fitterv1.Station{ExerciseId: exerciseID(t, deps, key)}
	mod(st)
	return st
}

func f64(v float64) *float64 { return &v }
func i32(v int32) *int32     { return &v }
func i64(v int64) *int64     { return &v }

func circuit(blocks ...*fitterv1.Block) *fitterv1.ManualActivity {
	a := manual()
	a.SportType = "Crossfit"
	a.Blocks = blocks
	return a
}

func TestBlocks_SupersetAndHyroxStations(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	acts := service.NewActivityService(deps)
	goals := service.NewGoalService(deps)
	ctx := ctxAs(alice.ID)

	a := circuit(
		&fitterv1.Block{Rounds: 3, Stations: []*fitterv1.Station{
			stationOf(deps, t, "bench press", func(s *fitterv1.Station) { s.WeightKg, s.Reps = f64(70), i32(8) }),
			stationOf(deps, t, "bench press", func(s *fitterv1.Station) { s.WeightKg, s.Reps = f64(80), i32(5) }),
			stationOf(deps, t, "pull-up", func(s *fitterv1.Station) { s.Reps = i32(8) }),
		}},
		&fitterv1.Block{Rounds: 1, Stations: []*fitterv1.Station{
			stationOf(deps, t, "skierg", func(s *fitterv1.Station) { s.DistanceM = f64(1000) }),
			stationOf(deps, t, "sled push", func(s *fitterv1.Station) { s.DistanceM, s.WeightKg = f64(50), f64(100) }),
			stationOf(deps, t, "plank", func(s *fitterv1.Station) { s.DurationS = i64(90) }),
		}},
	)
	resp, err := acts.CreateManualActivity(ctx, connect.NewRequest(&fitterv1.CreateManualActivityRequest{Activity: a}))
	if err != nil {
		t.Fatalf("CreateManualActivity: %v", err)
	}
	got := resp.Msg.GetActivity()
	if len(got.GetBlocks()) != 2 || got.GetBlocks()[0].GetRounds() != 3 || len(got.GetBlocks()[0].GetStations()) != 3 || got.GetBlocks()[0].GetStations()[2].GetExerciseName() != "Pull-up" {
		t.Fatalf("blocks = %v", got.GetBlocks())
	}
	// The previous release reads the heaviest weight × reps lift per exercise, so it is dual-written.
	if l := got.GetLifts(); len(l) != 1 || l[0].GetExercise() != "bench press" || l[0].GetWeightKg() != 80 || l[0].GetReps() != 5 {
		t.Fatalf("legacy lifts = %v, want bench press 80 kg × 5 only", l)
	}

	// A loaded distance station counts for a max-weight goal on that exercise.
	sled := "sled push"
	start, end := time.Now().Add(-24*time.Hour).Unix(), time.Now().Add(24*time.Hour).Unix()
	g, err := goals.CreateGoal(ctx, connect.NewRequest(&fitterv1.CreateGoalRequest{
		Name: "Push 150", PeriodStart: ts(start), PeriodEnd: ts(end),
		Clauses: []*fitterv1.GoalClause{{
			Metric: fitterv1.ActivityMetric_ACTIVITY_METRIC_MAX_WEIGHT_KG, Scope: fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
			Comparator: fitterv1.Comparator_COMPARATOR_GTE, Value: 150, Exercise: &sled,
		}},
	}))
	if err != nil {
		t.Fatalf("CreateGoal: %v", err)
	}
	if v := g.Msg.GetGoal().GetProgress().GetClauses()[0].GetCurrentValue(); v != 100 {
		t.Fatalf("sled push max = %v, want 100", v)
	}

	// Update is a full replace of the blocks and the dual-written lifts.
	got.Blocks = []*fitterv1.Block{{Rounds: 2, Stations: []*fitterv1.Station{stationOf(deps, t, "squat", func(s *fitterv1.Station) { s.WeightKg, s.Reps = f64(100), i32(5) })}}}
	up, err := acts.UpdateManualActivity(ctx, connect.NewRequest(&fitterv1.UpdateManualActivityRequest{Activity: got}))
	if err != nil {
		t.Fatalf("UpdateManualActivity: %v", err)
	}
	if b := up.Msg.GetActivity().GetBlocks(); len(b) != 1 || b[0].GetRounds() != 2 {
		t.Fatalf("after update blocks = %v", b)
	}
	if l := up.Msg.GetActivity().GetLifts(); len(l) != 1 || l[0].GetExercise() != "squat" {
		t.Fatalf("after update lifts = %v", l)
	}
}

func TestBlocks_Validation(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	acts := service.NewActivityService(deps)
	one := func(rounds int32, st ...*fitterv1.Station) *fitterv1.ManualActivity {
		return circuit(&fitterv1.Block{Rounds: rounds, Stations: st})
	}
	for name, a := range map[string]*fitterv1.ManualActivity{
		"weight_reps without weight":  one(1, stationOf(deps, t, "bench press", func(s *fitterv1.Station) { s.Reps = i32(5) })),
		"weight_reps with distance":   one(1, stationOf(deps, t, "bench press", func(s *fitterv1.Station) { s.WeightKg, s.Reps, s.DistanceM = f64(80), i32(5), f64(5) })),
		"reps with weight":            one(1, stationOf(deps, t, "pull-up", func(s *fitterv1.Station) { s.Reps, s.WeightKg = i32(5), f64(10) })),
		"distance with reps":          one(1, stationOf(deps, t, "skierg", func(s *fitterv1.Station) { s.DistanceM, s.Reps = f64(1000), i32(3) })),
		"weight on unloaded distance": one(1, stationOf(deps, t, "skierg", func(s *fitterv1.Station) { s.DistanceM, s.WeightKg = f64(1000), f64(5) })),
		"time without duration":       one(1, stationOf(deps, t, "plank", func(s *fitterv1.Station) {})),
		"nan weight":                  one(1, stationOf(deps, t, "bench press", func(s *fitterv1.Station) { s.WeightKg, s.Reps = f64(math.NaN()), i32(5) })),
		"huge distance":               one(1, stationOf(deps, t, "skierg", func(s *fitterv1.Station) { s.DistanceM = f64(1e7) })),
		"zero rounds":                 one(0, stationOf(deps, t, "pull-up", func(s *fitterv1.Station) { s.Reps = i32(5) })),
		"too many rounds":             one(101, stationOf(deps, t, "pull-up", func(s *fitterv1.Station) { s.Reps = i32(5) })),
		"empty block":                 one(1),
		"unknown exercise":            one(1, &fitterv1.Station{ExerciseId: "nope", Reps: i32(5)}),
	} {
		_, err := acts.CreateManualActivity(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateManualActivityRequest{Activity: a}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}
	// Blocks only belong to strength and circuits sports.
	run := one(1, stationOf(deps, t, "pull-up", func(s *fitterv1.Station) { s.Reps = i32(5) }))
	run.SportType = "Run"
	if _, err := acts.CreateManualActivity(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateManualActivityRequest{Activity: run})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("run with blocks: got %v, want InvalidArgument", err)
	}
}

// The previous release's UI sends lifts only; they become 1-round blocks and an unknown name joins the library.
func TestBlocks_LegacyLiftsRequest(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	acts := service.NewActivityService(deps)
	resp, err := acts.CreateManualActivity(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateManualActivityRequest{
		Activity: manual(&fitterv1.Lift{Exercise: "Bench Press", WeightKg: 85, Reps: 5}, &fitterv1.Lift{Exercise: "Zottman  curl", WeightKg: 12, Reps: 10}),
	}))
	if err != nil {
		t.Fatalf("CreateManualActivity: %v", err)
	}
	b := resp.Msg.GetActivity().GetBlocks()
	if len(b) != 2 || b[0].GetRounds() != 1 || b[0].GetStations()[0].GetExerciseName() != "Bench press" || b[1].GetStations()[0].GetExerciseName() != "zottman curl" {
		t.Fatalf("blocks = %v", b)
	}
	e, err := deps.Queries.GetExerciseByKey(context.Background(), "zottman curl")
	if err != nil || len(e.Muscles) != 0 || e.Measure != "weight_reps" {
		t.Fatalf("new exercise = %+v err=%v, want weight_reps with no muscles", e, err)
	}
}

func TestAdminExercises_UpdateDeleteMerge(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	admin := seedUser(t, deps.Queries, "admin")
	acts := service.NewActivityService(deps)
	goals := service.NewGoalService(deps)
	svc := service.NewAdminService(deps)
	actx, mctx := ctxAsAdmin(admin.ID), ctxAs(alice.ID)

	// Members can't touch the library's structure.
	sq := exerciseID(t, deps, "squat")
	if _, err := svc.UpdateExercise(mctx, connect.NewRequest(&fitterv1.UpdateExerciseRequest{Id: sq})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("member update: got %v, want PermissionDenied", err)
	}
	if _, err := svc.DeleteExercise(mctx, connect.NewRequest(&fitterv1.DeleteExerciseRequest{Id: sq})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("member delete: got %v, want PermissionDenied", err)
	}
	if _, err := svc.MergeExercises(mctx, connect.NewRequest(&fitterv1.MergeExercisesRequest{SourceId: sq, TargetId: exerciseID(t, deps, "front squat")})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("member merge: got %v, want PermissionDenied", err)
	}

	// Alice logs a squat and sets a goal on it.
	if _, err := acts.CreateManualActivity(mctx, connect.NewRequest(&fitterv1.CreateManualActivityRequest{
		Activity: circuit(&fitterv1.Block{Rounds: 1, Stations: []*fitterv1.Station{stationOf(deps, t, "squat", func(s *fitterv1.Station) { s.WeightKg, s.Reps = f64(100), i32(5) })}}),
	})); err != nil {
		t.Fatalf("log: %v", err)
	}
	squat := "squat"
	start, end := time.Now().Add(-24*time.Hour).Unix(), time.Now().Add(24*time.Hour).Unix()
	if _, err := goals.CreateGoal(mctx, connect.NewRequest(&fitterv1.CreateGoalRequest{
		Name: "Squat 140", PeriodStart: ts(start), PeriodEnd: ts(end),
		Clauses: []*fitterv1.GoalClause{{
			Metric: fitterv1.ActivityMetric_ACTIVITY_METRIC_MAX_WEIGHT_KG, Scope: fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
			Comparator: fitterv1.Comparator_COMPARATOR_GTE, Value: 140, Exercise: &squat,
		}},
	})); err != nil {
		t.Fatalf("CreateGoal: %v", err)
	}
	clauseExercise := func() string {
		var ex string
		if err := deps.DB.QueryRow(`SELECT exercise FROM goal_clauses LIMIT 1`).Scan(&ex); err != nil {
			t.Fatal(err)
		}
		return ex
	}

	// Used exercises can't change how they're measured or be deleted.
	if _, err := svc.UpdateExercise(actx, connect.NewRequest(&fitterv1.UpdateExerciseRequest{
		Id: sq, Name: "Squat", Measure: fitterv1.ExerciseMeasure_EXERCISE_MEASURE_REPS, Muscles: []fitterv1.Muscle{fitterv1.Muscle_MUSCLE_QUADS},
	})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("measure change while used: got %v, want InvalidArgument", err)
	}
	if _, err := svc.DeleteExercise(actx, connect.NewRequest(&fitterv1.DeleteExerciseRequest{Id: sq})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("delete while used: got %v, want FailedPrecondition", err)
	}
	// A new name that collides with another exercise is refused.
	if _, err := svc.UpdateExercise(actx, connect.NewRequest(&fitterv1.UpdateExerciseRequest{
		Id: sq, Name: "Front Squat", Measure: fitterv1.ExerciseMeasure_EXERCISE_MEASURE_WEIGHT_REPS, Muscles: []fitterv1.Muscle{fitterv1.Muscle_MUSCLE_QUADS},
	})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("colliding rename: got %v, want InvalidArgument", err)
	}

	// Renaming rewrites the goal clause's key and the legacy lift row.
	up, err := svc.UpdateExercise(actx, connect.NewRequest(&fitterv1.UpdateExerciseRequest{
		Id: sq, Name: "Back squat", Measure: fitterv1.ExerciseMeasure_EXERCISE_MEASURE_WEIGHT_REPS, Muscles: []fitterv1.Muscle{fitterv1.Muscle_MUSCLE_QUADS, fitterv1.Muscle_MUSCLE_GLUTES},
	}))
	if err != nil || up.Msg.GetExercise().GetKey() != "back squat" || up.Msg.GetExercise().GetUseCount() != 1 {
		t.Fatalf("rename: %+v err=%v", up.Msg, err)
	}
	var legacy string
	if err := deps.DB.QueryRow(`SELECT exercise FROM activity_lifts LIMIT 1`).Scan(&legacy); err != nil || legacy != "back squat" || clauseExercise() != "back squat" {
		t.Fatalf("legacy=%q clause=%q err=%v, want both rewritten to back squat", legacy, clauseExercise(), err)
	}

	// Merging moves stations and the clause, and removes the source.
	front := exerciseID(t, deps, "front squat")
	if _, err := svc.MergeExercises(actx, connect.NewRequest(&fitterv1.MergeExercisesRequest{SourceId: sq, TargetId: exerciseID(t, deps, "plank")})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("merge across measures: got %v, want InvalidArgument", err)
	}
	m, err := svc.MergeExercises(actx, connect.NewRequest(&fitterv1.MergeExercisesRequest{SourceId: sq, TargetId: front}))
	if err != nil || m.Msg.GetExercise().GetId() != front || m.Msg.GetExercise().GetUseCount() != 1 {
		t.Fatalf("merge: %+v err=%v", m.Msg, err)
	}
	if _, err := deps.Queries.GetExercise(context.Background(), sq); err == nil {
		t.Fatal("source exercise still exists after merge")
	}
	if clauseExercise() != "front squat" {
		t.Fatalf("clause = %q, want front squat", clauseExercise())
	}

	// An unused exercise deletes cleanly, and every admin change is audited.
	made, err := service.NewExerciseService(deps).CreateExercise(mctx, connect.NewRequest(newExercise(fitterv1.ExerciseMeasure_EXERCISE_MEASURE_TIME, fitterv1.Muscle_MUSCLE_CORE)))
	if err != nil {
		t.Fatalf("CreateExercise: %v", err)
	}
	if _, err := svc.DeleteExercise(actx, connect.NewRequest(&fitterv1.DeleteExerciseRequest{Id: made.Msg.GetExercise().GetId()})); err != nil {
		t.Fatalf("delete unused: %v", err)
	}
	if _, err := svc.DeleteExercise(actx, connect.NewRequest(&fitterv1.DeleteExerciseRequest{Id: made.Msg.GetExercise().GetId()})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("delete twice: got %v, want NotFound", err)
	}
	var actions int
	if err := deps.DB.QueryRow(`SELECT count(*) FROM admin_actions WHERE action IN ('update_exercise','merge_exercise','delete_exercise')`).Scan(&actions); err != nil || actions != 3 {
		t.Fatalf("audited actions = %d err=%v, want 3 (rename, merge, delete)", actions, err)
	}
}
