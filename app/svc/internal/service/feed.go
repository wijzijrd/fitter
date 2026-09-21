package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/progress"
	"github.com/fitter/svc/internal/sports"
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
			UserId:           r.UserID,
			UserName:         r.UserName,
			UserAvatarUrl:    r.UserAvatarUrl,
			Type:             r.Type,
			DistanceM:        r.DistanceM,
			ElevationM:       r.ElevationM,
			MovingTimeS:      r.MovingTimeS,
			StartDate:        unixToTimestamp(r.StartDate),
			Name:             r.Name,
			StravaActivityId: r.StravaActivityID,
			SportType:        r.SportType,
			SportGroup:       string(sports.GroupOf(r.SportType, r.Type)),
			AverageHeartrate: nullFloatPtr(r.AverageHeartrate),
			AverageWatts:     nullFloatPtr(r.AverageWatts),
			SufferScore:      nullFloatPtr(r.SufferScore),
		})
	}
	return connect.NewResponse(&fitterv1.GetCampaignFeedResponse{Activities: out}), nil
}

// pooledActivities returns every campaign member's activities across the
// campaign's window (ordered by start_date ASC), sport-agnostic — used by
// both the leaderboard and the timeline's pooled total, which apply
// club-wide regardless of any individual member's goal sport.
func (s *FeedService) pooledActivities(ctx context.Context, c db.Campaign) ([]metrics.Activity, error) {
	members, err := s.Queries.ListCampaignMembers(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	userIDs := make([]string, len(members))
	for i, m := range members {
		userIDs[i] = m.UserID
	}
	rows, err := s.Queries.ListActivitiesForUsersInRange(ctx, db.ListActivitiesForUsersInRangeParams{
		UserIds:     userIDs,
		StartDate:   c.StartDate,
		StartDate_2: c.EndDate,
	})
	if err != nil {
		return nil, err
	}
	acts := make([]metrics.Activity, len(rows))
	for i, r := range rows {
		acts[i] = metrics.FromDB(r)
	}
	return acts, nil
}

// pooledTotal folds every member activity in the campaign window through
// the campaign metric's Accumulator, for use by both the leaderboard's
// pooled_progress_value and the timeline's running pooled series.
func (s *FeedService) pooledTotal(ctx context.Context, c db.Campaign) (def metrics.Def, acts []metrics.Activity, err error) {
	key := metrics.Key(c.Metric.String)
	def, ok := metrics.Lookup(key)
	if !ok {
		return metrics.Def{}, nil, fmt.Errorf("campaign %s: unknown pooled metric %q", c.ID, c.Metric.String)
	}
	acts, err = s.pooledActivities(ctx, c)
	if err != nil {
		return metrics.Def{}, nil, err
	}
	return def, acts, nil
}

func (s *FeedService) GetCampaignLeaderboard(ctx context.Context, req *connect.Request[fitterv1.GetCampaignLeaderboardRequest]) (*connect.Response[fitterv1.GetCampaignLeaderboardResponse], error) {
	c, _, err := s.requireMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}

	goalRows, err := s.Queries.ListCampaignGoalsWithUsers(ctx, sql.NullString{String: c.ID, Valid: true})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	goals := make([]db.Goal, len(goalRows))
	userNameByGoal := make(map[string]string, len(goalRows))
	avatarByGoal := make(map[string]string, len(goalRows))
	goalIDs := make([]string, len(goalRows))
	for i, r := range goalRows {
		goals[i] = db.Goal{
			ID:          r.ID,
			UserID:      r.UserID,
			CampaignID:  r.CampaignID,
			Name:        r.Name,
			SportGroup:  r.SportGroup,
			SportType:   r.SportType,
			PeriodStart: r.PeriodStart,
			PeriodEnd:   r.PeriodEnd,
			CreatedAt:   r.CreatedAt,
		}
		userNameByGoal[r.ID] = r.UserName
		avatarByGoal[r.ID] = r.UserAvatarUrl
		goalIDs[i] = r.ID
	}

	clauseRows, err := s.Queries.ListGoalClausesForGoals(ctx, goalIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	clausesByGoal := make(map[string][]db.GoalClause, len(goals))
	for _, cl := range clauseRows {
		clausesByGoal[cl.GoalID] = append(clausesByGoal[cl.GoalID], cl)
	}
	progressClauses := make(map[string][]progress.Clause, len(goals))
	for gid, cls := range clausesByGoal {
		pc, err := clausesToProgress(cls)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		progressClauses[gid] = pc
	}

	results, err := s.evaluateGoals(ctx, goals, progressClauses)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	entriesByGroup := make(map[string][]*fitterv1.LeaderboardEntry)
	for _, g := range goals {
		res := results[g.ID]
		entriesByGroup[goalGroupKey(g.SportGroup, g.SportType)] = append(entriesByGroup[goalGroupKey(g.SportGroup, g.SportType)], &fitterv1.LeaderboardEntry{
			UserId:                  g.UserID,
			UserName:                userNameByGoal[g.ID],
			UserAvatarUrl:           avatarByGoal[g.ID],
			GoalId:                  g.ID,
			GoalName:                g.Name,
			ProgressRatio:           res.Ratio,
			Satisfied:               res.Satisfied,
			Clauses:                 clauseResultsToProto(res.Clauses),
			ConsideredActivityCount: int32(res.ConsideredCount),
		})
	}

	order := sportGroupOrder()
	keys := make([]string, 0, len(entriesByGroup))
	for k := range entriesByGroup {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return order[keys[i]] < order[keys[j]] })

	groups := make([]*fitterv1.LeaderboardGroup, 0, len(keys))
	for _, k := range keys {
		entries := entriesByGroup[k]
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].ProgressRatio != entries[j].ProgressRatio {
				return entries[i].ProgressRatio > entries[j].ProgressRatio
			}
			if entries[i].Satisfied != entries[j].Satisfied {
				return entries[i].Satisfied
			}
			return entries[i].UserName < entries[j].UserName
		})
		for i, e := range entries {
			e.Rank = int32(i + 1)
		}
		groups = append(groups, &fitterv1.LeaderboardGroup{
			SportGroup:      k,
			SportGroupLabel: sportGroupLabel(k),
			Entries:         entries,
		})
	}

	resp := &fitterv1.GetCampaignLeaderboardResponse{Groups: groups}
	if c.TargetValue.Valid {
		def, acts, err := s.pooledTotal(ctx, c)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		acc := def.NewAcc()
		for _, a := range acts {
			acc.Add(a)
		}
		value, _, _ := acc.Result()
		target := c.TargetValue.Float64
		metricProto := keyToMetric(metrics.Key(c.Metric.String))
		resp.PooledProgressValue = &value
		resp.PooledTargetValue = &target
		resp.PooledMetric = &metricProto
	}

	return connect.NewResponse(resp), nil
}

func (s *FeedService) GetCampaignTimeline(ctx context.Context, req *connect.Request[fitterv1.GetCampaignTimelineRequest]) (*connect.Response[fitterv1.GetCampaignTimelineResponse], error) {
	c, _, err := s.requireMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}

	goalRows, err := s.Queries.ListCampaignGoalsWithUsers(ctx, sql.NullString{String: c.ID, Valid: true})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	goals := make([]db.Goal, len(goalRows))
	userNameByGoal := make(map[string]string, len(goalRows))
	goalIDs := make([]string, len(goalRows))
	for i, r := range goalRows {
		goals[i] = db.Goal{
			ID:          r.ID,
			UserID:      r.UserID,
			CampaignID:  r.CampaignID,
			Name:        r.Name,
			SportGroup:  r.SportGroup,
			SportType:   r.SportType,
			PeriodStart: r.PeriodStart,
			PeriodEnd:   r.PeriodEnd,
			CreatedAt:   r.CreatedAt,
		}
		userNameByGoal[r.ID] = r.UserName
		goalIDs[i] = r.ID
	}

	clauseRows, err := s.Queries.ListGoalClausesForGoals(ctx, goalIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	clausesByGoal := make(map[string][]db.GoalClause, len(goals))
	for _, cl := range clauseRows {
		clausesByGoal[cl.GoalID] = append(clausesByGoal[cl.GoalID], cl)
	}

	actsByUser, err := s.batchActivitiesByUser(ctx, goals)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	seriesByGroup := make(map[string][]*fitterv1.TimelineSeries)
	for _, g := range goals {
		sel := sports.Selector{Group: sports.Group(g.SportGroup.String), SportType: g.SportType.String}
		acts := actsByUser[g.UserID]
		filtered := make([]metrics.Activity, 0, len(acts))
		for _, a := range acts {
			if sports.Matches(sel, a.SportType, a.Type) && g.PeriodStart <= a.StartDate && a.StartDate <= g.PeriodEnd {
				filtered = append(filtered, a)
			}
		}

		for _, cl := range clausesByGoal[g.ID] {
			// SINGLE-scope clauses aren't cumulative; their state is already
			// fully captured in the leaderboard's ClauseProgress, so they
			// produce no timeline series.
			if cl.Scope != "aggregate" {
				continue
			}
			metricKey := metrics.Key(cl.Metric)
			def, ok := metrics.Lookup(metricKey)
			if !ok {
				// Defensive only: persisted clause rows have already passed
				// validateClauses. Skip rather than panic on a data bug.
				continue
			}
			series := &fitterv1.TimelineSeries{
				UserId:      g.UserID,
				Label:       userNameByGoal[g.ID],
				GoalId:      g.ID,
				ClauseId:    cl.ID,
				Metric:      keyToMetric(metricKey),
				Comparator:  comparatorFromDB(cl.Comparator),
				TargetValue: cl.Value,
			}
			acc := def.NewAcc()
			for _, a := range filtered {
				acc.Add(a)
				value, _, ok := acc.Result()
				if !ok {
					// e.g. a nullable metric before any activity has
					// supplied it yet — skip rather than plot a phantom
					// zero.
					continue
				}
				series.Points = append(series.Points, &fitterv1.TimelinePoint{
					Date:  unixToTimestamp(a.StartDate),
					Value: value,
				})
			}
			key := goalGroupKey(g.SportGroup, g.SportType)
			seriesByGroup[key] = append(seriesByGroup[key], series)
		}
	}

	order := sportGroupOrder()
	keys := make([]string, 0, len(seriesByGroup))
	for k := range seriesByGroup {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return order[keys[i]] < order[keys[j]] })

	groups := make([]*fitterv1.TimelineGroup, 0, len(keys))
	for _, k := range keys {
		groups = append(groups, &fitterv1.TimelineGroup{
			SportGroup:      k,
			SportGroupLabel: sportGroupLabel(k),
			Series:          seriesByGroup[k],
		})
	}

	var pooled *fitterv1.TimelineSeries
	if c.TargetValue.Valid {
		def, acts, err := s.pooledTotal(ctx, c)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		pooled = &fitterv1.TimelineSeries{
			Label:       "Total",
			Metric:      keyToMetric(metrics.Key(c.Metric.String)),
			TargetValue: c.TargetValue.Float64,
		}
		acc := def.NewAcc()
		for _, a := range acts {
			acc.Add(a)
			value, _, ok := acc.Result()
			if !ok {
				continue
			}
			pooled.Points = append(pooled.Points, &fitterv1.TimelinePoint{
				Date:  unixToTimestamp(a.StartDate),
				Value: value,
			})
		}
	}

	return connect.NewResponse(&fitterv1.GetCampaignTimelineResponse{
		Groups:    groups,
		Pooled:    pooled,
		StartDate: unixToTimestamp(c.StartDate),
		EndDate:   unixToTimestamp(c.EndDate),
	}), nil
}
