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
)

const (
	maxDistanceM   = 1_000_000.0
	maxMovingTimeS = 7 * 24 * 3600
	maxWeightKg    = 1000.0
	maxReps        = 1000
	maxLifts       = 50
)

var minStart = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

func invalid(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}

// validateManual checks a ManualActivity from a request and returns its
// lifts normalised. Lifts only make sense for strength sports.
func validateManual(a *fitterv1.ManualActivity) ([]*fitterv1.Lift, error) {
	if a.GetSportType() == "" {
		return nil, invalid("sport_type is required")
	}
	if off := a.GetUtcOffsetMinutes(); off < -14*60 || off > 14*60 {
		return nil, invalid("utc_offset_minutes out of range")
	}
	if a.GetStartDate() == nil {
		return nil, invalid("start_date is required")
	}
	// Written as !(x >= 0 && x <= max) so NaN fails too.
	if d := a.GetDistanceM(); !(d >= 0 && d <= maxDistanceM) {
		return nil, invalid("distance must be between 0 and 1000 km")
	}
	if t := a.GetMovingTimeS(); t < 0 || t > maxMovingTimeS {
		return nil, invalid("time must be between 0 and 7 days")
	}
	// A day of slack covers time zones ahead of the server.
	if start := a.GetStartDate().AsTime(); start.After(time.Now().Add(24*time.Hour)) || start.Before(minStart) {
		return nil, invalid("start_date must be in the past")
	}
	if len(a.GetLifts()) > maxLifts {
		return nil, invalid("too many lifts")
	}
	if len(a.GetLifts()) > 0 && sports.GroupOf(a.GetSportType(), a.GetSportType()) != sports.GroupStrength {
		return nil, invalid("lifts are only for strength activities")
	}
	lifts := make([]*fitterv1.Lift, 0, len(a.GetLifts()))
	for _, l := range a.GetLifts() {
		ex := normalizeExercise(l.GetExercise())
		w := l.GetWeightKg()
		if ex == "" || len(ex) > 60 || math.IsNaN(w) || !(w > 0 && w <= maxWeightKg) || l.GetReps() <= 0 || l.GetReps() > maxReps {
			return nil, invalid("each lift needs an exercise, a weight up to 1000 kg and 1 to 1000 reps")
		}
		lifts = append(lifts, &fitterv1.Lift{Exercise: ex, WeightKg: l.GetWeightKg(), Reps: l.GetReps()})
	}
	return lifts, nil
}

func (s *ActivityService) toProto(ctx context.Context, q *db.Queries, rows []db.Activity) ([]*fitterv1.ManualActivity, error) {
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.StravaActivityID
	}
	liftRows := []db.ListLiftsForActivitiesRow{}
	if len(ids) > 0 {
		var err error
		if liftRows, err = q.ListLiftsForActivities(ctx, ids); err != nil {
			return nil, err
		}
	}
	liftsByID := map[int64][]*fitterv1.Lift{}
	for _, l := range liftRows {
		liftsByID[l.ActivityID] = append(liftsByID[l.ActivityID], &fitterv1.Lift{Exercise: l.Exercise, WeightKg: l.WeightKg, Reps: l.Reps})
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
	lifts, err := validateManual(a)
	if err != nil {
		return nil, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
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
	for _, l := range lifts {
		if err := q.CreateLift(ctx, db.CreateLiftParams{ActivityID: row.StravaActivityID, Exercise: l.Exercise, WeightKg: l.WeightKg, Reps: l.Reps}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
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
	lifts, err := validateManual(a)
	if err != nil {
		return nil, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
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
	if err := q.DeleteLiftsForActivity(ctx, row.StravaActivityID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, l := range lifts {
		if err := q.CreateLift(ctx, db.CreateLiftParams{ActivityID: row.StravaActivityID, Exercise: l.Exercise, WeightKg: l.WeightKg, Reps: l.Reps}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
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
