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
	"github.com/google/uuid"
)

func toCampaignProto(c db.Campaign, joined bool, memberCount int64) *fitterv1.Campaign {
	return &fitterv1.Campaign{
		Id:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		Mode:        campaignModeFromDB(c.Mode),
		Metric:      metricFromDB(c.Metric),
		TargetValue: c.TargetValue,
		StartDate:   unixToTimestamp(c.StartDate),
		EndDate:     unixToTimestamp(c.EndDate),
		CreatedBy:   c.CreatedBy,
		CreatedAt:   unixToTimestamp(c.CreatedAt),
		Joined:      joined,
		MemberCount: int32(memberCount),
	}
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
	mode, err := campaignModeToDB(msg.GetMode())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	metric, err := metricToDB(msg.GetMetric())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.GetTargetValue() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_value must be positive"))
	}

	now := time.Now().Unix()
	c, err := s.Queries.CreateCampaign(ctx, db.CreateCampaignParams{
		ID:          uuid.NewString(),
		Name:        msg.GetName(),
		Description: msg.GetDescription(),
		Mode:        mode,
		Metric:      metric,
		TargetValue: msg.GetTargetValue(),
		StartDate:   timestampToUnix(msg.GetStartDate()),
		EndDate:     timestampToUnix(msg.GetEndDate()),
		CreatedBy:   userID,
		CreatedAt:   now,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// The creator automatically joins their own campaign.
	if err := s.Queries.JoinCampaign(ctx, db.JoinCampaignParams{
		CampaignID: c.ID,
		UserID:     userID,
		JoinedAt:   now,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&fitterv1.CreateCampaignResponse{
		Campaign: toCampaignProto(c, true, 1),
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
		out = append(out, toCampaignProto(c, slices.Contains(joinedIDs, c.ID), memberCount))
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
	return connect.NewResponse(&fitterv1.GetCampaignResponse{
		Campaign: toCampaignProto(c, joined, memberCount),
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

	perMemberTarget := sql.NullFloat64{}
	if c.Mode == "per_member" {
		v := c.TargetValue
		if req.Msg.PerMemberTargetValue != nil {
			v = req.Msg.GetPerMemberTargetValue()
		}
		perMemberTarget = sql.NullFloat64{Float64: v, Valid: true}
	}

	if err := s.Queries.JoinCampaign(ctx, db.JoinCampaignParams{
		CampaignID:           c.ID,
		UserID:               userID,
		JoinedAt:             time.Now().Unix(),
		PerMemberTargetValue: perMemberTarget,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	memberCount, err := s.Queries.CountCampaignMembers(ctx, c.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.JoinCampaignResponse{
		Campaign: toCampaignProto(c, true, memberCount),
	}), nil
}

func (s *CampaignService) LeaveCampaign(ctx context.Context, req *connect.Request[fitterv1.LeaveCampaignRequest]) (*connect.Response[fitterv1.LeaveCampaignResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	if err := s.Queries.LeaveCampaign(ctx, db.LeaveCampaignParams{
		CampaignID: req.Msg.GetCampaignId(),
		UserID:     userID,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.LeaveCampaignResponse{}), nil
}
