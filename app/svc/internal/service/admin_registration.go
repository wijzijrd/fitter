package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
)

// auditedTx runs fn and its admin_actions row in one tx, after checking the caller is an admin.
func (s *AdminService) auditedTx(ctx context.Context, action, target, detail string, fn func(q *db.Queries) error) error {
	if err := requireAdmin(ctx); err != nil {
		return err
	}
	actor, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return internalErr(err)
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
	if err := fn(q); err != nil {
		return internalErr(err)
	}
	if err := audit(ctx, q, actor, action, target, detail); err != nil {
		return internalErr(err)
	}
	if err := tx.Commit(); err != nil {
		return internalErr(err)
	}
	return nil
}

func (s *AdminService) GetRegistration(ctx context.Context, _ *connect.Request[fitterv1.GetRegistrationRequest]) (*connect.Response[fitterv1.GetRegistrationResponse], error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	mode, err := s.Queries.GetRegistrationMode(ctx)
	if err != nil {
		return nil, internalErr(err)
	}
	rows, err := s.Queries.ListRegistrationAllowlist(ctx)
	if err != nil {
		return nil, internalErr(err)
	}
	allowed := make([]*fitterv1.RegistrationAllowance, len(rows))
	for i, r := range rows {
		allowed[i] = &fitterv1.RegistrationAllowance{StravaAthleteId: r.StravaAthleteID, AddedAt: unixToTimestamp(r.AddedAt)}
	}
	return connect.NewResponse(&fitterv1.GetRegistrationResponse{Mode: mode, Allowed: allowed}), nil
}

func (s *AdminService) SetRegistrationMode(ctx context.Context, req *connect.Request[fitterv1.SetRegistrationModeRequest]) (*connect.Response[fitterv1.SetRegistrationModeResponse], error) {
	mode := req.Msg.GetMode()
	if mode != "open" && mode != "invite_only" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("mode must be open or invite_only"))
	}
	err := s.auditedTx(ctx, "registration_mode", "registration", mode, func(q *db.Queries) error {
		return q.SetRegistrationMode(ctx, mode)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&fitterv1.SetRegistrationModeResponse{}), nil
}

func (s *AdminService) AllowAthlete(ctx context.Context, req *connect.Request[fitterv1.AllowAthleteRequest]) (*connect.Response[fitterv1.AllowAthleteResponse], error) {
	id := req.Msg.GetStravaAthleteId()
	if id <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("strava athlete id must be a positive number"))
	}
	actor, _ := auth.UserIDFromContext(ctx)
	err := s.auditedTx(ctx, "allow_athlete", strconv.FormatInt(id, 10), "", func(q *db.Queries) error {
		return q.AddToRegistrationAllowlist(ctx, db.AddToRegistrationAllowlistParams{StravaAthleteID: id, AddedBy: actor, AddedAt: time.Now().Unix()})
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&fitterv1.AllowAthleteResponse{}), nil
}

func (s *AdminService) RevokeAthlete(ctx context.Context, req *connect.Request[fitterv1.RevokeAthleteRequest]) (*connect.Response[fitterv1.RevokeAthleteResponse], error) {
	id := req.Msg.GetStravaAthleteId()
	err := s.auditedTx(ctx, "revoke_athlete", strconv.FormatInt(id, 10), "", func(q *db.Queries) error {
		return q.RemoveFromRegistrationAllowlist(ctx, id)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&fitterv1.RevokeAthleteResponse{}), nil
}
