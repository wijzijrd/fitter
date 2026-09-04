package service

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
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
	return connect.NewResponse(&fitterv1.GetCurrentUserResponse{
		User: &fitterv1.User{
			Id:        u.ID,
			Name:      u.Name,
			AvatarUrl: u.AvatarUrl,
		},
	}), nil
}
