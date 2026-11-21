package service

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/google/uuid"
)

const maxBanReasonLen = 500

func audit(ctx context.Context, q *db.Queries, actor, action, target, detail string) error {
	return q.InsertAdminAction(ctx, db.InsertAdminActionParams{
		ID: uuid.NewString(), ActorID: actor, Action: action, TargetID: target, Detail: detail,
		CreatedAt: time.Now().Unix(),
	})
}

func adminUserToProto(u db.User) *fitterv1.AdminUser {
	out := &fitterv1.AdminUser{
		Id: u.ID, Name: u.Name, AvatarUrl: u.AvatarUrl, StravaAthleteId: u.StravaAthleteID,
		Role: u.Role, Status: u.Status, BanReason: u.BanReason, CreatedAt: unixToTimestamp(u.CreatedAt),
	}
	if u.BannedBy.Valid {
		out.BannedBy = u.BannedBy.String
	}
	if u.BannedAt.Valid {
		out.BannedAt = unixToTimestamp(u.BannedAt.Int64)
	}
	return out
}

func (s *AdminService) ListUsers(ctx context.Context, req *connect.Request[fitterv1.ListUsersRequest]) (*connect.Response[fitterv1.ListUsersResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	var rows []db.User
	var err error
	if st := req.Msg.GetStatus(); st != "" {
		rows, err = s.Queries.ListUsersByStatus(ctx, st)
	} else {
		rows, err = s.Queries.ListAllUsers(ctx)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*fitterv1.AdminUser, len(rows))
	for i, u := range rows {
		out[i] = adminUserToProto(u)
	}
	return connect.NewResponse(&fitterv1.ListUsersResponse{Users: out}), nil
}

// mutateUser runs fn plus its audit row in one tx. fn receives the target as
// read inside the tx and returns the audit action/detail.
func (s *AdminService) mutateUser(ctx context.Context, targetID string, fn func(q *db.Queries, actor string, target db.User) (action, detail string, err error)) error {
	if err := requireAdmin(ctx); err != nil {
		return err
	}
	actor, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
	target, err := q.GetUserByID(ctx, targetID)
	if errors.Is(err, sql.ErrNoRows) {
		return connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	action, detail, err := fn(q, actor, target)
	if err != nil {
		return err
	}
	if err := audit(ctx, q, actor, action, targetID, detail); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if err := tx.Commit(); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	return nil
}

func internalErr(err error) error { return connect.NewError(connect.CodeInternal, err) }

func (s *AdminService) ApproveUser(ctx context.Context, req *connect.Request[fitterv1.ApproveUserRequest]) (*connect.Response[fitterv1.ApproveUserResponse], error) {
	err := s.mutateUser(ctx, req.Msg.GetId(), func(q *db.Queries, _ string, t db.User) (string, string, error) {
		if t.Status != "pending" {
			return "", "", connect.NewError(connect.CodeFailedPrecondition, errors.New("user is not pending"))
		}
		if err := q.SetUserStatus(ctx, db.SetUserStatusParams{ID: t.ID, Status: "active"}); err != nil {
			return "", "", internalErr(err)
		}
		return "approve", t.Name, nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&fitterv1.ApproveUserResponse{}), nil
}

func (s *AdminService) BanUser(ctx context.Context, req *connect.Request[fitterv1.BanUserRequest]) (*connect.Response[fitterv1.BanUserResponse], error) {
	reason := req.Msg.GetReason()
	if reason == "" || utf8.RuneCountInString(reason) > maxBanReasonLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason is required and must be at most 500 characters"))
	}
	err := s.mutateUser(ctx, req.Msg.GetId(), func(q *db.Queries, actor string, t db.User) (string, string, error) {
		if t.ID == actor {
			return "", "", connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot ban yourself"))
		}
		if t.Role == "admin" {
			return "", "", connect.NewError(connect.CodeFailedPrecondition, errors.New("demote the admin before banning"))
		}
		if t.Status == "banned" {
			return "", "", connect.NewError(connect.CodeFailedPrecondition, errors.New("user already banned"))
		}
		if err := q.BanUser(ctx, db.BanUserParams{
			ID: t.ID, BanReason: reason,
			BannedBy: sql.NullString{String: actor, Valid: true},
			BannedAt: sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		}); err != nil {
			return "", "", internalErr(err)
		}
		return "ban", reason, nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&fitterv1.BanUserResponse{}), nil
}

func (s *AdminService) UnbanUser(ctx context.Context, req *connect.Request[fitterv1.UnbanUserRequest]) (*connect.Response[fitterv1.UnbanUserResponse], error) {
	err := s.mutateUser(ctx, req.Msg.GetId(), func(q *db.Queries, _ string, t db.User) (string, string, error) {
		if t.Status != "banned" {
			return "", "", connect.NewError(connect.CodeFailedPrecondition, errors.New("user is not banned"))
		}
		if err := q.UnbanUser(ctx, t.ID); err != nil {
			return "", "", internalErr(err)
		}
		return "unban", t.Name, nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&fitterv1.UnbanUserResponse{}), nil
}

func (s *AdminService) SetUserRole(ctx context.Context, req *connect.Request[fitterv1.SetUserRoleRequest]) (*connect.Response[fitterv1.SetUserRoleResponse], error) {
	role := req.Msg.GetRole()
	if role != "admin" && role != "member" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role must be admin or member"))
	}
	err := s.mutateUser(ctx, req.Msg.GetId(), func(q *db.Queries, actor string, t db.User) (string, string, error) {
		if t.Role == role {
			return "", "", connect.NewError(connect.CodeFailedPrecondition, errors.New("user already has that role"))
		}
		action := "promote"
		if role == "member" {
			action = "demote"
			if t.ID == actor {
				return "", "", connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot demote yourself"))
			}
			n, err := q.CountActiveAdmins(ctx)
			if err != nil {
				return "", "", internalErr(err)
			}
			if t.Status == "active" && n <= 1 {
				return "", "", connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot demote the last active admin"))
			}
		} else if t.Status != "active" {
			return "", "", connect.NewError(connect.CodeFailedPrecondition, errors.New("only active users can be promoted"))
		}
		if err := q.SetUserRole(ctx, db.SetUserRoleParams{ID: t.ID, Role: role}); err != nil {
			return "", "", internalErr(err)
		}
		return action, t.Name, nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&fitterv1.SetUserRoleResponse{}), nil
}

func (s *AdminService) ListAdminActions(ctx context.Context, req *connect.Request[fitterv1.ListAdminActionsRequest]) (*connect.Response[fitterv1.ListAdminActionsResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	limit := req.Msg.GetLimit()
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := s.Queries.ListAdminActions(ctx, limit)
	if err != nil {
		return nil, internalErr(err)
	}
	out := make([]*fitterv1.AdminAction, len(rows))
	for i, r := range rows {
		out[i] = &fitterv1.AdminAction{
			Id: r.ID, ActorId: r.ActorID, ActorName: r.ActorName, Action: r.Action,
			TargetId: r.TargetID, Detail: r.Detail, CreatedAt: unixToTimestamp(r.CreatedAt),
		}
	}
	return connect.NewResponse(&fitterv1.ListAdminActionsResponse{Actions: out}), nil
}
