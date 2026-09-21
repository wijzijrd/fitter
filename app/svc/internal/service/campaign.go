package service

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/metrics"
	"github.com/google/uuid"
)

// toCampaignProto maps a persisted campaign row to the wire Campaign
// message. Metric/TargetValue are populated only when the row's
// target_value is valid — a campaign with no pooled target leaves both
// unset (CreateCampaign enforces they're set together, never just one).
func toCampaignProto(c db.Campaign, joined bool, memberCount, goalCount int64) *fitterv1.Campaign {
	out := &fitterv1.Campaign{
		Id:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		StartDate:   unixToTimestamp(c.StartDate),
		EndDate:     unixToTimestamp(c.EndDate),
		CreatedBy:   c.CreatedBy,
		CreatedAt:   unixToTimestamp(c.CreatedAt),
		Joined:      joined,
		MemberCount: int32(memberCount),
		GoalCount:   int32(goalCount),
	}
	if c.TargetValue.Valid {
		m := keyToMetric(metrics.Key(c.Metric.String))
		v := c.TargetValue.Float64
		out.Metric = &m
		out.TargetValue = &v
	}
	return out
}

// joinCampaignTx upserts userID's membership in campaign c and, in the same
// transaction, creates each of goalMsgs as a real goal via createGoalInTx —
// the same validated creation path CreateGoal uses — with campaign_id and
// the period forced to c regardless of what each request carries. Either
// the membership row and every goal are created, or none of them are: a
// failure partway through (e.g. an invalid clause on the second goal) rolls
// back the membership row too, so a caller can never end up a member of a
// campaign whose requested goals only partially exist.
//
// Called with an empty goalMsgs by CreateCampaign's creator auto-join,
// which degrades correctly to a plain membership insert — a campaign with a
// zero-goal creator-member is valid; they add goals afterward via CreateGoal
// with campaign_id set.
func (d *Deps) joinCampaignTx(ctx context.Context, userID string, c db.Campaign, goalMsgs []*fitterv1.CreateGoalRequest) ([]db.Goal, map[string][]db.GoalClause, error) {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	qtx := d.Queries.WithTx(tx)

	if err := qtx.JoinCampaign(ctx, db.JoinCampaignParams{
		CampaignID: c.ID,
		UserID:     userID,
		JoinedAt:   time.Now().Unix(),
	}); err != nil {
		return nil, nil, connect.NewError(connect.CodeInternal, err)
	}

	goals := make([]db.Goal, len(goalMsgs))
	clausesByGoal := make(map[string][]db.GoalClause, len(goalMsgs))
	for i, gm := range goalMsgs {
		g, clauses, err := d.createGoalInTx(ctx, qtx, userID, gm, &c)
		if err != nil {
			return nil, nil, err
		}
		goals[i] = g
		clausesByGoal[g.ID] = clauses
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	return goals, clausesByGoal, nil
}

func (s *CampaignService) CreateCampaign(ctx context.Context, req *connect.Request[fitterv1.CreateCampaignRequest]) (*connect.Response[fitterv1.CreateCampaignResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	msg := req.Msg
	if msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	hasMetric := msg.Metric != nil
	hasTarget := msg.TargetValue != nil
	if hasMetric != hasTarget {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("metric and target_value must both be set or both unset"))
	}
	var metricCol sql.NullString
	var targetCol sql.NullFloat64
	if hasMetric {
		if msg.GetTargetValue() <= 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_value must be positive"))
		}
		key, err := metricToKey(msg.GetMetric())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		metricCol = sql.NullString{String: string(key), Valid: true}
		targetCol = sql.NullFloat64{Float64: msg.GetTargetValue(), Valid: true}
	}

	startDate := timestampToUnix(msg.GetStartDate())
	endDate := timestampToUnix(msg.GetEndDate())
	if endDate <= startDate {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("end_date must be after start_date"))
	}

	c, err := s.Queries.CreateCampaign(ctx, db.CreateCampaignParams{
		ID:          uuid.NewString(),
		Name:        msg.GetName(),
		Description: msg.GetDescription(),
		Metric:      metricCol,
		TargetValue: targetCol,
		StartDate:   startDate,
		EndDate:     endDate,
		CreatedBy:   userID,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// The creator automatically joins their own campaign, via the same
	// shared join logic JoinCampaign uses, with no goals.
	if _, _, err := s.joinCampaignTx(ctx, userID, c, nil); err != nil {
		return nil, err
	}

	return connect.NewResponse(&fitterv1.CreateCampaignResponse{
		Campaign: toCampaignProto(c, true, 1, 0),
	}), nil
}

func (s *CampaignService) ListCampaigns(ctx context.Context, req *connect.Request[fitterv1.ListCampaignsRequest]) (*connect.Response[fitterv1.ListCampaignsResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	campaigns, err := s.Queries.ListCampaigns(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	joinedIDs, err := s.Queries.ListJoinedCampaignIDsForUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*fitterv1.Campaign, 0, len(campaigns))
	for _, c := range campaigns {
		memberCount, err := s.Queries.CountCampaignMembers(ctx, c.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		goalCount, err := s.Queries.CountCampaignGoals(ctx, sql.NullString{String: c.ID, Valid: true})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out = append(out, toCampaignProto(c, slices.Contains(joinedIDs, c.ID), memberCount, goalCount))
	}
	return connect.NewResponse(&fitterv1.ListCampaignsResponse{Campaigns: out}), nil
}

func (s *CampaignService) GetCampaign(ctx context.Context, req *connect.Request[fitterv1.GetCampaignRequest]) (*connect.Response[fitterv1.GetCampaignResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	c, err := s.Queries.GetCampaign(ctx, req.Msg.GetCampaignId())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("campaign not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	joined, err := s.Queries.IsCampaignMember(ctx, db.IsCampaignMemberParams{CampaignID: c.ID, UserID: userID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	memberCount, err := s.Queries.CountCampaignMembers(ctx, c.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	goalCount, err := s.Queries.CountCampaignGoals(ctx, sql.NullString{String: c.ID, Valid: true})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.GetCampaignResponse{
		Campaign: toCampaignProto(c, joined, memberCount, goalCount),
	}), nil
}

func (s *CampaignService) JoinCampaign(ctx context.Context, req *connect.Request[fitterv1.JoinCampaignRequest]) (*connect.Response[fitterv1.JoinCampaignResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	c, err := s.Queries.GetCampaign(ctx, req.Msg.GetCampaignId())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("campaign not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	goals, clausesByGoal, err := s.joinCampaignTx(ctx, userID, c, req.Msg.GetGoals())
	if err != nil {
		return nil, err
	}

	memberCount, err := s.Queries.CountCampaignMembers(ctx, c.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	goalCount, err := s.Queries.CountCampaignGoals(ctx, sql.NullString{String: c.ID, Valid: true})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoGoals, err := s.toGoalProtosWithProgress(ctx, goals, clausesByGoal)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&fitterv1.JoinCampaignResponse{
		Campaign: toCampaignProto(c, true, memberCount, goalCount),
		Goals:    protoGoals,
	}), nil
}

func (s *CampaignService) LeaveCampaign(ctx context.Context, req *connect.Request[fitterv1.LeaveCampaignRequest]) (*connect.Response[fitterv1.LeaveCampaignResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	qtx := s.Queries.WithTx(tx)

	// Leaving discards the goals defined for this campaign — they have no
	// meaning outside it.
	if err := qtx.DeleteGoalsForUserInCampaign(ctx, db.DeleteGoalsForUserInCampaignParams{
		CampaignID: sql.NullString{String: req.Msg.GetCampaignId(), Valid: true},
		UserID:     userID,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := qtx.LeaveCampaign(ctx, db.LeaveCampaignParams{
		CampaignID: req.Msg.GetCampaignId(),
		UserID:     userID,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.LeaveCampaignResponse{}), nil
}
