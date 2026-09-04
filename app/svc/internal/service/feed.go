package service

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
)

// requireMember loads the campaign and errors unless the current user is a member —
// campaign feed/leaderboard/timeline are visible only to joined members.
func (s *FeedService) requireMember(ctx context.Context, campaignID string) (db.Campaign, string, error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return db.Campaign{}, "", connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	c, err := s.Queries.GetCampaign(ctx, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Campaign{}, "", connect.NewError(connect.CodeNotFound, errors.New("campaign not found"))
	}
	if err != nil {
		return db.Campaign{}, "", connect.NewError(connect.CodeInternal, err)
	}
	joined, err := s.Queries.IsCampaignMember(ctx, db.IsCampaignMemberParams{CampaignID: campaignID, UserID: userID})
	if err != nil {
		return db.Campaign{}, "", connect.NewError(connect.CodeInternal, err)
	}
	if !joined {
		return db.Campaign{}, "", connect.NewError(connect.CodePermissionDenied, errors.New("must join campaign to view it"))
	}
	return c, userID, nil
}

func (s *FeedService) GetCampaignFeed(ctx context.Context, req *connect.Request[fitterv1.GetCampaignFeedRequest]) (*connect.Response[fitterv1.GetCampaignFeedResponse], error) {
	c, _, err := s.requireMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	rows, err := s.Queries.ListCampaignFeedActivities(ctx, db.ListCampaignFeedActivitiesParams{
		CampaignID:  c.ID,
		StartDate:   c.StartDate,
		StartDate_2: c.EndDate,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*fitterv1.FeedActivity, 0, len(rows))
	for _, r := range rows {
		out = append(out, &fitterv1.FeedActivity{
			Id:            r.ID,
			UserId:        r.UserID,
			UserName:      r.UserName,
			UserAvatarUrl: r.UserAvatarUrl,
			Type:          r.Type,
			DistanceM:     r.DistanceM,
			ElevationM:    r.ElevationM,
			MovingTimeS:   r.MovingTimeS,
			StartDate:     unixToTimestamp(r.StartDate),
			Name:          r.Name,
		})
	}
	return connect.NewResponse(&fitterv1.GetCampaignFeedResponse{Activities: out}), nil
}

// memberTarget returns the target this member is measured against: their
// per-member override if set, else the campaign's default target.
func memberTarget(c db.Campaign, m db.ListCampaignMembersRow) float64 {
	if m.PerMemberTargetValue.Valid {
		return m.PerMemberTargetValue.Float64
	}
	return c.TargetValue
}

func (s *FeedService) campaignActivities(ctx context.Context, c db.Campaign, userIDs []string) ([]db.Activity, error) {
	return s.Queries.ListActivitiesForUsersInRange(ctx, db.ListActivitiesForUsersInRangeParams{
		UserIds:     userIDs,
		StartDate:   c.StartDate,
		StartDate_2: c.EndDate,
	})
}

func (s *FeedService) GetCampaignLeaderboard(ctx context.Context, req *connect.Request[fitterv1.GetCampaignLeaderboardRequest]) (*connect.Response[fitterv1.GetCampaignLeaderboardResponse], error) {
	c, _, err := s.requireMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	members, err := s.Queries.ListCampaignMembers(ctx, c.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	userIDs := make([]string, len(members))
	for i, m := range members {
		userIDs[i] = m.UserID
	}
	activities, err := s.campaignActivities(ctx, c, userIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	progressByUser := make(map[string]float64, len(members))
	for _, a := range activities {
		progressByUser[a.UserID] += activityMetricValue(a, c.Metric)
	}

	entries := make([]*fitterv1.LeaderboardEntry, 0, len(members))
	var pooledTotal float64
	for _, m := range members {
		progress := progressByUser[m.UserID]
		pooledTotal += progress
		target := memberTarget(c, m)
		ratio := 0.0
		if target > 0 {
			ratio = progress / target
		}
		entries = append(entries, &fitterv1.LeaderboardEntry{
			UserId:        m.UserID,
			UserName:      m.UserName,
			UserAvatarUrl: m.UserAvatarUrl,
			ProgressValue: progress,
			TargetValue:   target,
			ProgressRatio: ratio,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ProgressValue > entries[j].ProgressValue })
	for i, e := range entries {
		e.Rank = int32(i + 1)
	}

	resp := &fitterv1.GetCampaignLeaderboardResponse{Entries: entries}
	if c.Mode == "pooled" {
		resp.PooledProgressValue = pooledTotal
	}
	return connect.NewResponse(resp), nil
}

func (s *FeedService) GetCampaignTimeline(ctx context.Context, req *connect.Request[fitterv1.GetCampaignTimelineRequest]) (*connect.Response[fitterv1.GetCampaignTimelineResponse], error) {
	c, _, err := s.requireMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	members, err := s.Queries.ListCampaignMembers(ctx, c.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	userIDs := make([]string, len(members))
	for i, m := range members {
		userIDs[i] = m.UserID
	}
	// activities come back ordered by start_date ASC, which is what the
	// cumulative-sum walk below relies on.
	activities, err := s.campaignActivities(ctx, c, userIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var series []*fitterv1.TimelineSeries
	if c.Mode == "pooled" {
		pooled := &fitterv1.TimelineSeries{Label: "Total"}
		var running float64
		for _, a := range activities {
			running += activityMetricValue(a, c.Metric)
			pooled.Points = append(pooled.Points, &fitterv1.TimelinePoint{
				Date:            unixToTimestamp(a.StartDate),
				CumulativeValue: running,
			})
		}
		series = []*fitterv1.TimelineSeries{pooled}
	} else {
		byUser := make(map[string]*fitterv1.TimelineSeries, len(members))
		running := make(map[string]float64, len(members))
		for _, m := range members {
			byUser[m.UserID] = &fitterv1.TimelineSeries{UserId: m.UserID, Label: m.UserName}
		}
		for _, a := range activities {
			running[a.UserID] += activityMetricValue(a, c.Metric)
			ts := byUser[a.UserID]
			ts.Points = append(ts.Points, &fitterv1.TimelinePoint{
				Date:            unixToTimestamp(a.StartDate),
				CumulativeValue: running[a.UserID],
			})
		}
		for _, uid := range userIDs {
			series = append(series, byUser[uid])
		}
	}

	return connect.NewResponse(&fitterv1.GetCampaignTimelineResponse{
		Series:      series,
		TargetValue: c.TargetValue,
		StartDate:   unixToTimestamp(c.StartDate),
		EndDate:     unixToTimestamp(c.EndDate),
	}), nil
}
