package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/ingest"
)

const (
	syncMinInterval = 5 * time.Minute
	// Strava lists by start time, so an activity that syncs from a watch hours late starts before
	// the newest one we hold. Looking back a week catches it; re-upserting the overlap is idempotent.
	syncLookback = 7 * 24 * time.Hour
)

func (s *ActivityService) SyncActivities(ctx context.Context, _ *connect.Request[fitterv1.SyncActivitiesRequest]) (*connect.Response[fitterv1.SyncActivitiesResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	now := time.Now()
	u, err := s.Queries.ClaimUserSync(ctx, db.ClaimUserSyncParams{
		ID: userID, LastSyncedAt: now.Unix(), LastSyncedAt_2: now.Add(-syncMinInterval).Unix(),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return connect.NewResponse(&fitterv1.SyncActivitiesResponse{Throttled: true}), nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	before, err := s.Queries.CountActivitiesForUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	latest, err := s.Queries.GetLatestActivityStartDateForUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	after := latest - int64(syncLookback/time.Second)
	if after < 0 {
		after = 0
	}
	if _, err := ingest.Backfill(ctx, s.Queries, s.Strava, u, after); errors.Is(err, ingest.ErrNotConnected) {
		return connect.NewResponse(&fitterv1.SyncActivitiesResponse{}), nil // signing in fetches history
	} else if err != nil {
		slog.Error("sync activities failed", "user_id", userID, "err", err)
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("couldn't reach Strava"))
	}
	n, err := s.Queries.CountActivitiesForUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.SyncActivitiesResponse{Imported: int32(n - before)}), nil
}
