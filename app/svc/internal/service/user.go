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

func (s *UserService) GetCurrentUser(ctx context.Context, req *connect.Request[fitterv1.GetCurrentUserRequest]) (*connect.Response[fitterv1.GetCurrentUserResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	u, err := s.Queries.GetUserByID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.GetCurrentUserResponse{User: currentUserToProto(u)}), nil
}

func currentUserToProto(u db.User) *fitterv1.User {
	return &fitterv1.User{
		Id:        u.ID,
		Name:      u.Name,
		AvatarUrl: u.AvatarUrl,
		Role:      u.Role,
		Status:    u.Status,
		FeedSettings: &fitterv1.FeedSettings{
			ShowPace: u.FeedShowPace, ShowPower: u.FeedShowPower, ShowEffort: u.FeedShowEffort, ShowLifts: u.FeedShowLifts,
		},
	}
}

// UpdateFeedSettings replaces the caller's four feed display choices.
func (s *UserService) UpdateFeedSettings(ctx context.Context, req *connect.Request[fitterv1.UpdateFeedSettingsRequest]) (*connect.Response[fitterv1.UpdateFeedSettingsResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	f := req.Msg.GetFeedSettings()
	u, err := s.Queries.SetUserFeedSettings(ctx, db.SetUserFeedSettingsParams{
		ID: userID, FeedShowPace: f.GetShowPace(), FeedShowPower: f.GetShowPower(), FeedShowEffort: f.GetShowEffort(), FeedShowLifts: f.GetShowLifts(),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.UpdateFeedSettingsResponse{User: currentUserToProto(u)}), nil
}
