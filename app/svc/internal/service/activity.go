package service

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/sports"
	"github.com/google/uuid"
)

const (
	maxDistanceM   = 1_000_000.0
	maxMovingTimeS = 7 * 24 * 3600
	maxWeightKg    = 1000.0
	maxReps        = 1000
	maxLifts       = 50 // legacy lifts list only

	maxBlocks           = 30
	maxStationsPerBlock = 20
	maxStations         = 100
	maxRounds           = 100
	maxStationDistanceM = 100_000.0
	maxStationDurationS = 24 * 3600
)

var minStart = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

func invalid(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}

// validateManual checks the scalar fields of a ManualActivity from a request. Exercise
// blocks need the library, so planBlocks checks those.
func validateManual(a *fitterv1.ManualActivity) error {
	if a.GetSportType() == "" {
		return invalid("sport_type is required")
	}
	if off := a.GetUtcOffsetMinutes(); off < -14*60 || off > 14*60 {
		return invalid("utc_offset_minutes out of range")
	}
	if a.GetStartDate() == nil {
		return invalid("start_date is required")
	}
	// Written as !(x >= 0 && x <= max) so NaN fails too.
	if d := a.GetDistanceM(); !(d >= 0 && d <= maxDistanceM) {
		return invalid("distance must be between 0 and 1000 km")
	}
	if t := a.GetMovingTimeS(); t < 0 || t > maxMovingTimeS {
		return invalid("time must be between 0 and 7 days")
	}
	// A day of slack covers time zones ahead of the server.
	if start := a.GetStartDate().AsTime(); start.After(time.Now().Add(24*time.Hour)) || start.Before(minStart) {
		return invalid("start_date must be in the past")
	}
	return nil
}

// plannedStation is a validated station with its exercise resolved.
type plannedStation struct {
	exercise db.Exercise
	weight   sql.NullFloat64
	reps     sql.NullInt32
	distance sql.NullFloat64
	duration sql.NullInt64
}

type plannedBlock struct {
	rounds   int32
	stations []plannedStation
}

// blocksAllowed: only lifting and HIIT/circuit sessions carry exercises.
func blocksAllowed(sportType string) bool {
	g := sports.GroupOf(sportType, sportType)
	return g == sports.GroupStrength || g == sports.GroupCircuits
}

// planBlocks validates the request's exercises and resolves them against the library, using
// q (the transaction's queries when writing). A request with blocks wins over the deprecated
// lifts list; lifts alone (the previous release's UI) become one 1-round block each.
func planBlocks(ctx context.Context, q *db.Queries, userID string, a *fitterv1.ManualActivity) ([]plannedBlock, error) {
	if len(a.GetBlocks()) == 0 && len(a.GetLifts()) == 0 {
		return nil, nil
	}
	if !blocksAllowed(a.GetSportType()) {
		return nil, invalid("exercises are only for strength and HIIT & circuits activities")
	}
	if len(a.GetBlocks()) > 0 {
		return planFromBlocks(ctx, q, a.GetBlocks())
	}
	return planFromLifts(ctx, q, userID, a.GetLifts())
}

func planFromBlocks(ctx context.Context, q *db.Queries, blocks []*fitterv1.Block) ([]plannedBlock, error) {
	if len(blocks) > maxBlocks {
		return nil, invalid("too many blocks")
	}
	total := 0
	var ids []string
	for _, b := range blocks {
		if b.GetRounds() < 1 || b.GetRounds() > maxRounds {
			return nil, invalid("rounds must be 1 to 100")
		}
		if n := len(b.GetStations()); n < 1 || n > maxStationsPerBlock {
			return nil, invalid("each block needs 1 to 20 exercises")
		}
		total += len(b.GetStations())
		for _, st := range b.GetStations() {
			ids = append(ids, st.GetExerciseId())
		}
	}
	if total > maxStations {
		return nil, invalid("too many exercises in one session")
	}
	found, err := q.GetExercisesByIDs(ctx, ids)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	byID := make(map[string]db.Exercise, len(found))
	for _, e := range found {
		byID[e.ID] = e
	}
	out := make([]plannedBlock, len(blocks))
	for i, b := range blocks {
		out[i].rounds = b.GetRounds()
		for _, st := range b.GetStations() {
			ex, ok := byID[st.GetExerciseId()]
			if !ok {
				return nil, invalid("unknown exercise")
			}
			if ex.ArchivedAt.Valid {
				return nil, invalid(ex.Name + " has been retired; pick another exercise")
			}
			ps, err := planStation(ex, st)
			if err != nil {
				return nil, err
			}
			out[i].stations = append(out[i].stations, ps)
		}
	}
	return out, nil
}

// planStation checks a station's result against its exercise's measure: the fields the
// measure needs are set and in range, and no other field is.
func planStation(ex db.Exercise, st *fitterv1.Station) (plannedStation, error) {
	hasW, hasR, hasD, hasT := st.WeightKg != nil, st.Reps != nil, st.DistanceM != nil, st.DurationS != nil
	w, r, d, t := st.GetWeightKg(), st.GetReps(), st.GetDistanceM(), st.GetDurationS()
	// Written as !(x > 0 && x <= max) so NaN fails too.
	okW := w > 0 && w <= maxWeightKg
	okR := r >= 1 && r <= maxReps
	okD := d > 0 && d <= maxStationDistanceM
	okT := t >= 1 && t <= maxStationDurationS
	bad := func(need string) (plannedStation, error) {
		return plannedStation{}, invalid(ex.Name + " needs " + need)
	}
	p := plannedStation{exercise: ex}
	switch ex.Measure {
	case "weight_reps":
		if !hasW || !hasR || hasD || hasT || !okW || !okR {
			return bad("a weight up to 1000 kg and 1 to 1000 reps")
		}
	case "reps":
		if !hasR || hasW || hasD || hasT || !okR {
			return bad("1 to 1000 reps")
		}
	case "distance":
		if !hasD || hasR || hasT || !okD || (hasW && (!ex.Loaded || !okW)) {
			return bad("a distance up to 100 km" + optionalWeight(ex))
		}
	case "time":
		if !hasT || hasR || hasD || !okT || (hasW && (!ex.Loaded || !okW)) {
			return bad("a time up to 24 hours" + optionalWeight(ex))
		}
	default:
		return bad("a valid measure")
	}
	if hasW {
		p.weight = sql.NullFloat64{Float64: w, Valid: true}
	}
	if hasR {
		p.reps = sql.NullInt32{Int32: r, Valid: true}
	}
	if hasD {
		p.distance = sql.NullFloat64{Float64: d, Valid: true}
	}
	if hasT {
		p.duration = sql.NullInt64{Int64: t, Valid: true}
	}
	return p, nil
}

func optionalWeight(ex db.Exercise) string {
	if ex.Loaded {
		return " (and optionally a weight up to 1000 kg)"
	}
	return ""
}

// planFromLifts converts the previous release's flat lifts into 1-round blocks. A name not
// yet in the library is added as a weight × reps exercise with no muscles.
func planFromLifts(ctx context.Context, q *db.Queries, userID string, lifts []*fitterv1.Lift) ([]plannedBlock, error) {
	if len(lifts) > maxLifts {
		return nil, invalid("too many lifts")
	}
	out := make([]plannedBlock, 0, len(lifts))
	for _, l := range lifts {
		key := normalizeExercise(l.GetExercise())
		w := l.GetWeightKg()
		if key == "" || len(key) > maxExerciseNameLen || math.IsNaN(w) || !(w > 0 && w <= maxWeightKg) || l.GetReps() <= 0 || l.GetReps() > maxReps {
			return nil, invalid("each lift needs an exercise, a weight up to 1000 kg and 1 to 1000 reps")
		}
		ex, err := q.GetExerciseByKey(ctx, key)
		if errors.Is(err, sql.ErrNoRows) {
			ex, err = q.CreateExercise(ctx, db.CreateExerciseParams{
				ID: uuid.NewString(), Name: key, Key: key, Measure: "weight_reps", Muscles: []string{},
				CreatedBy: sql.NullString{String: userID, Valid: true}, CreatedAt: time.Now().Unix(),
			})
		}
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if ex.ArchivedAt.Valid {
			return nil, invalid(ex.Name + " has been retired; pick another exercise")
		}
		ps, err := planStation(ex, &fitterv1.Station{WeightKg: &w, Reps: &l.Reps})
		if err != nil {
			return nil, err
		}
		out = append(out, plannedBlock{rounds: 1, stations: []plannedStation{ps}})
	}
	return out, nil
}

// legacyLift is one row of the old activity_lifts table.
type legacyLift struct {
	exercise string
	weightKg float64
	reps     int32
}

// legacyLifts derives what the previous release reads: the heaviest weight (and its reps)
// per weight × reps exercise, in first-seen order.
func legacyLifts(blocks []plannedBlock) []legacyLift {
	var out []legacyLift
	idx := map[string]int{}
	for _, b := range blocks {
		for _, st := range b.stations {
			if st.exercise.Measure != "weight_reps" {
				continue
			}
			i, ok := idx[st.exercise.Key]
			if !ok {
				idx[st.exercise.Key] = len(out)
				out = append(out, legacyLift{st.exercise.Key, st.weight.Float64, st.reps.Int32})
			} else if st.weight.Float64 > out[i].weightKg {
				out[i] = legacyLift{st.exercise.Key, st.weight.Float64, st.reps.Int32}
			}
		}
	}
	return out
}

// writeBlocks stores the blocks and dual-writes activity_lifts, so containers from the
// previous release still read lifts back while old and new overlap.
// shortcut: drop the dual-write and activity_lifts in the release after this one is everywhere.
func writeBlocks(ctx context.Context, q *db.Queries, activityID int64, blocks []plannedBlock) error {
	for bi, b := range blocks {
		blockID, err := q.CreateBlock(ctx, db.CreateBlockParams{ActivityID: activityID, Position: int32(bi), Rounds: b.rounds})
		if err != nil {
			return err
		}
		for si, st := range b.stations {
			if err := q.CreateStation(ctx, db.CreateStationParams{
				BlockID: blockID, Position: int32(si), ExerciseID: st.exercise.ID,
				WeightKg: st.weight, Reps: st.reps, DistanceM: st.distance, DurationS: st.duration,
			}); err != nil {
				return err
			}
		}
	}
	for _, l := range legacyLifts(blocks) {
		if err := q.CreateLift(ctx, db.CreateLiftParams{ActivityID: activityID, Exercise: l.exercise, WeightKg: l.weightKg, Reps: l.reps}); err != nil {
			return err
		}
	}
	return nil
}

func nullIntPtr[T int32 | int64](v T, ok bool) *T {
	if !ok {
		return nil
	}
	return &v
}

func (s *ActivityService) toProto(ctx context.Context, q *db.Queries, rows []db.Activity) ([]*fitterv1.ManualActivity, error) {
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.StravaActivityID
	}
	var (
		liftRows    []db.ListLiftsForActivitiesRow
		blockRows   []db.ActivityBlock
		stationRows []db.ListStationsForActivitiesRow
	)
	if len(ids) > 0 {
		var err error
		if liftRows, err = q.ListLiftsForActivities(ctx, ids); err != nil {
			return nil, err
		}
		if blockRows, err = q.ListBlocksForActivities(ctx, ids); err != nil {
			return nil, err
		}
		if stationRows, err = q.ListStationsForActivities(ctx, ids); err != nil {
			return nil, err
		}
	}
	liftsByID := map[int64][]*fitterv1.Lift{}
	for _, l := range liftRows {
		liftsByID[l.ActivityID] = append(liftsByID[l.ActivityID], &fitterv1.Lift{Exercise: l.Exercise, WeightKg: l.WeightKg, Reps: l.Reps})
	}
	stationsByBlock := map[int64][]*fitterv1.Station{}
	for _, st := range stationRows {
		stationsByBlock[st.BlockID] = append(stationsByBlock[st.BlockID], &fitterv1.Station{
			ExerciseId: st.ExerciseID, ExerciseName: st.ExerciseName,
			WeightKg: nullFloatPtr(st.WeightKg), Reps: nullIntPtr(st.Reps.Int32, st.Reps.Valid),
			DistanceM: nullFloatPtr(st.DistanceM), DurationS: nullIntPtr(st.DurationS.Int64, st.DurationS.Valid),
		})
	}
	blocksByID := map[int64][]*fitterv1.Block{}
	for _, b := range blockRows {
		blocksByID[b.ActivityID] = append(blocksByID[b.ActivityID], &fitterv1.Block{Rounds: b.Rounds, Stations: stationsByBlock[b.ID]})
	}
	// An activity written by a previous-release container has lifts but no blocks: show each
	// as a 1-round block. Those stations carry a name only (no exercise id).
	for id, lifts := range liftsByID {
		if len(blocksByID[id]) > 0 {
			continue
		}
		for _, l := range lifts {
			w, r := l.WeightKg, l.Reps
			blocksByID[id] = append(blocksByID[id], &fitterv1.Block{Rounds: 1, Stations: []*fitterv1.Station{{ExerciseName: l.Exercise, WeightKg: &w, Reps: &r}}})
		}
	}
	out := make([]*fitterv1.ManualActivity, len(rows))
	for i, r := range rows {
		m := &fitterv1.ManualActivity{
			Id:          r.StravaActivityID,
			SportType:   r.SportType,
			Name:        r.Name,
			StartDate:   unixToTimestamp(r.StartDate),
			MovingTimeS: r.MovingTimeS,
			Lifts:       liftsByID[r.StravaActivityID],
			Blocks:      blocksByID[r.StravaActivityID],
		}
		if r.DistanceM > 0 {
			d := r.DistanceM
			m.DistanceM = &d
		}
		out[i] = m
	}
	return out, nil
}

func (s *ActivityService) CreateManualActivity(ctx context.Context, req *connect.Request[fitterv1.CreateManualActivityRequest]) (*connect.Response[fitterv1.CreateManualActivityResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	a := req.Msg.GetActivity()
	if err := validateManual(a); err != nil {
		return nil, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
	blocks, err := planBlocks(ctx, q, userID, a)
	if err != nil {
		return nil, err
	}
	row, err := q.CreateManualActivity(ctx, db.CreateManualActivityParams{
		UserID:      userID,
		Type:        a.GetSportType(),
		Name:        a.GetName(),
		StartDate:   timestampToUnix(a.GetStartDate()),
		DistanceM:   a.GetDistanceM(),
		MovingTimeS: a.GetMovingTimeS(),
		CreatedAt:   time.Now().Unix(),

		StartDateLocal: localStart(a),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := writeBlocks(ctx, q, row.StravaActivityID, blocks); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out, err := s.toProto(ctx, s.Queries, []db.Activity{row})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.CreateManualActivityResponse{Activity: out[0]}), nil
}

func (s *ActivityService) UpdateManualActivity(ctx context.Context, req *connect.Request[fitterv1.UpdateManualActivityRequest]) (*connect.Response[fitterv1.UpdateManualActivityResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	a := req.Msg.GetActivity()
	if err := validateManual(a); err != nil {
		return nil, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
	blocks, err := planBlocks(ctx, q, userID, a)
	if err != nil {
		return nil, err
	}
	// The WHERE clause scopes to the owner and source='manual', so another
	// user's row (or a Strava one) reads as NotFound.
	row, err := q.UpdateManualActivity(ctx, db.UpdateManualActivityParams{
		StravaActivityID: a.GetId(),
		UserID:           userID,
		Type:             a.GetSportType(),
		Name:             a.GetName(),
		StartDate:        timestampToUnix(a.GetStartDate()),
		DistanceM:        a.GetDistanceM(),
		MovingTimeS:      a.GetMovingTimeS(),
		UpdatedAt:        time.Now().Unix(),
		StartDateLocal:   localStart(a),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("activity not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := q.DeleteBlocksForActivity(ctx, row.StravaActivityID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := q.DeleteLiftsForActivity(ctx, row.StravaActivityID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := writeBlocks(ctx, q, row.StravaActivityID, blocks); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out, err := s.toProto(ctx, s.Queries, []db.Activity{row})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.UpdateManualActivityResponse{Activity: out[0]}), nil
}

func (s *ActivityService) DeleteManualActivity(ctx context.Context, req *connect.Request[fitterv1.DeleteManualActivityRequest]) (*connect.Response[fitterv1.DeleteManualActivityResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	n, err := s.Queries.DeleteManualActivity(ctx, db.DeleteManualActivityParams{StravaActivityID: req.Msg.GetId(), UserID: userID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("activity not found"))
	}
	return connect.NewResponse(&fitterv1.DeleteManualActivityResponse{}), nil
}

func (s *ActivityService) ListManualActivities(ctx context.Context, _ *connect.Request[fitterv1.ListManualActivitiesRequest]) (*connect.Response[fitterv1.ListManualActivitiesResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	rows, err := s.Queries.ListManualActivitiesForUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out, err := s.toProto(ctx, s.Queries, rows)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.ListManualActivitiesResponse{Activities: out}), nil
}

// localStart is the wall-clock start as unix seconds-as-if-UTC, or NULL when
// the client didn't say its offset (evaluation then falls back to UTC).
func localStart(a *fitterv1.ManualActivity) sql.NullInt64 {
	if a.UtcOffsetMinutes == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: timestampToUnix(a.GetStartDate()) + int64(a.GetUtcOffsetMinutes())*60, Valid: true}
}
