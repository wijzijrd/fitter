package service

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/google/uuid"
	"time"
)

// goalProgress sums the owner's activities in [periodStart, periodEnd] for the goal's metric.
func (s *GoalService) goalProgress(ctx context.Context, userID, metric string, periodStart, periodEnd int64) (float64, error) {
	activities, err := s.Queries.ListActivitiesForUserInRange(ctx, db.ListActivitiesForUserInRangeParams{
		UserID:      userID,
		StartDate:   periodStart,
		StartDate_2: periodEnd,
	})
	if err != nil {
		return 0, err
	}
	var total float64
	for _, a := range activities {
		total += activityMetricValue(a, metric)
	}
	return total, nil
}

func (s *GoalService) toGoalProto(ctx context.Context, g db.Goal) (*fitterv1.Goal, error) {
	progress, err := s.goalProgress(ctx, g.UserID, g.Metric, g.PeriodStart, g.PeriodEnd)
	if err != nil {
		return nil, err
	}
	return &fitterv1.Goal{
		Id:            g.ID,
		Metric:        metricFromDB(g.Metric),
		TargetValue:   g.TargetValue,
		PeriodStart:   unixToTimestamp(g.PeriodStart),
		PeriodEnd:     unixToTimestamp(g.PeriodEnd),
		ProgressValue: progress,
		CreatedAt:     unixToTimestamp(g.CreatedAt),
	}, nil
}

func (s *GoalService) CreateGoal(ctx context.Context, req *connect.Request[fitterv1.CreateGoalRequest]) (*connect.Response[fitterv1.CreateGoalResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	msg := req.Msg
	metric, err := metricToDB(msg.GetMetric())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.GetTargetValue() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_value must be positive"))
	}
	periodStart := timestampToUnix(msg.GetPeriodStart())
	periodEnd := timestampToUnix(msg.GetPeriodEnd())
	if periodEnd <= periodStart {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("period_end must be after period_start"))
	}

	g, err := s.Queries.CreateGoal(ctx, db.CreateGoalParams{
		ID:          uuid.NewString(),
		UserID:      userID,
		Metric:      metric,
		TargetValue: msg.GetTargetValue(),
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	goal, err := s.toGoalProto(ctx, g)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.CreateGoalResponse{Goal: goal}), nil
}

func (s *GoalService) ListGoals(ctx context.Context, req *connect.Request[fitterv1.ListGoalsRequest]) (*connect.Response[fitterv1.ListGoalsResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	goals, err := s.Queries.ListGoalsForUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*fitterv1.Goal, 0, len(goals))
	for _, g := range goals {
		gp, err := s.toGoalProto(ctx, g)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out = append(out, gp)
	}
	return connect.NewResponse(&fitterv1.ListGoalsResponse{Goals: out}), nil
}

func (s *GoalService) UpdateGoal(ctx context.Context, req *connect.Request[fitterv1.UpdateGoalRequest]) (*connect.Response[fitterv1.UpdateGoalResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	msg := req.Msg
	if msg.GetTargetValue() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_value must be positive"))
	}
	periodStart := timestampToUnix(msg.GetPeriodStart())
	periodEnd := timestampToUnix(msg.GetPeriodEnd())
	if periodEnd <= periodStart {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("period_end must be after period_start"))
	}

	g, err := s.Queries.UpdateGoal(ctx, db.UpdateGoalParams{
		TargetValue: msg.GetTargetValue(),
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		ID:          msg.GetGoalId(),
		UserID:      userID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("goal not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	goal, err := s.toGoalProto(ctx, g)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.UpdateGoalResponse{Goal: goal}), nil
}

func (s *GoalService) DeleteGoal(ctx context.Context, req *connect.Request[fitterv1.DeleteGoalRequest]) (*connect.Response[fitterv1.DeleteGoalResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	if err := s.Queries.DeleteGoal(ctx, db.DeleteGoalParams{ID: req.Msg.GetGoalId(), UserID: userID}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.DeleteGoalResponse{}), nil
}
