package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/sports"
)

func invalid(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}

// validateManual checks a ManualActivity from a request and returns its
// lifts normalised. Lifts only make sense for strength sports.
func validateManual(a *fitterv1.ManualActivity) ([]*fitterv1.Lift, error) {
	if a.GetSportType() == "" {
		return nil, invalid("sport_type is required")
	}
	if a.GetStartDate() == nil {
		return nil, invalid("start_date is required")
	}
	if a.GetMovingTimeS() < 0 || a.GetDistanceM() < 0 {
		return nil, invalid("time and distance can't be negative")
	}
	if len(a.GetLifts()) > 0 && sports.GroupOf(a.GetSportType(), a.GetSportType()) != sports.GroupStrength {
		return nil, invalid("lifts are only for strength activities")
	}
	lifts := make([]*fitterv1.Lift, 0, len(a.GetLifts()))
	for _, l := range a.GetLifts() {
		ex := normalizeExercise(l.GetExercise())
		if ex == "" || l.GetWeightKg() <= 0 || l.GetReps() <= 0 {
			return nil, invalid("each lift needs an exercise, a weight above 0 and reps above 0")
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
