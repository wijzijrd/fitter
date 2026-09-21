package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/metrics"
	"github.com/fitter/svc/internal/progress"
	"github.com/fitter/svc/internal/sports"
	"github.com/google/uuid"
)

// preparedClause is a goal clause that has passed validation and is ready to
// insert (scope/comparator already translated to their DB string forms).
type preparedClause struct {
	scope, comparator string
	metricKey         metrics.Key
	value             float64
}

// validateClauses validates a full clause list from a Create/UpdateGoal
// request: at least one clause, every metric resolves, scope/comparator are
// specified, and SINGLE scope is rejected for a metric that doesn't support
// it (activity_count).
func validateClauses(clauses []*fitterv1.GoalClause) ([]preparedClause, error) {
	if len(clauses) == 0 {
		return nil, errors.New("at least one clause is required")
	}
	out := make([]preparedClause, len(clauses))
	for i, c := range clauses {
		metricKey, err := metricToKey(c.GetMetric())
		if err != nil {
			return nil, fmt.Errorf("clause %d: %w", i, err)
		}
		def, ok := metrics.Lookup(metricKey)
		if !ok {
			return nil, fmt.Errorf("clause %d: unknown metric %q", i, metricKey)
		}
		scope, err := scopeToDB(c.GetScope())
		if err != nil {
			return nil, fmt.Errorf("clause %d: %w", i, err)
		}
		if c.GetScope() == fitterv1.ClauseScope_CLAUSE_SCOPE_SINGLE && !def.SupportsSingle {
			return nil, fmt.Errorf("clause %d: metric %q does not support single scope", i, metricKey)
		}
		comparator, err := comparatorToDB(c.GetComparator())
		if err != nil {
			return nil, fmt.Errorf("clause %d: %w", i, err)
		}
		out[i] = preparedClause{scope: scope, comparator: comparator, metricKey: metricKey, value: c.GetValue()}
	}
	return out, nil
}

// clauseToProgress converts one persisted db.GoalClause into a
// progress.Clause for evaluation. The DB's CHECK constraints on scope and
// comparator mean an error here indicates a data-consistency bug, not bad
// user input.
func clauseToProgress(c db.GoalClause) (progress.Clause, error) {
	key := metrics.Key(c.Metric)
	if _, ok := metrics.Lookup(key); !ok {
		return progress.Clause{}, fmt.Errorf("goal_clauses row %s: unknown metric %q", c.ID, c.Metric)
	}
	var scope progress.Scope
	switch c.Scope {
	case "aggregate":
		scope = progress.ScopeAggregate
	case "single":
		scope = progress.ScopeSingle
	default:
		return progress.Clause{}, fmt.Errorf("goal_clauses row %s: unknown scope %q", c.ID, c.Scope)
	}
	var cmp progress.Comparator
	switch c.Comparator {
	case "gte":
		cmp = progress.GTE
	case "lte":
		cmp = progress.LTE
	default:
		return progress.Clause{}, fmt.Errorf("goal_clauses row %s: unknown comparator %q", c.ID, c.Comparator)
	}
	return progress.Clause{
		ID:         c.ID,
		Position:   int(c.Position),
		Scope:      scope,
		Metric:     key,
		Comparator: cmp,
		Value:      c.Value,
	}, nil
}

// clausesToProgress converts a slice of persisted clauses in one call.
func clausesToProgress(clauses []db.GoalClause) ([]progress.Clause, error) {
	out := make([]progress.Clause, len(clauses))
	for i, c := range clauses {
		pc, err := clauseToProgress(c)
		if err != nil {
			return nil, err
		}
		out[i] = pc
	}
	return out, nil
}

// clauseResultsToProto maps progress.ClauseResult (already in Position
// order) to the wire ClauseProgress list shared by Goal.progress and
// LeaderboardEntry.clauses.
func clauseResultsToProto(results []progress.ClauseResult) []*fitterv1.ClauseProgress {
	out := make([]*fitterv1.ClauseProgress, len(results))
	for i, cr := range results {
		out[i] = &fitterv1.ClauseProgress{
			ClauseId:                cr.ClauseID,
			Position:                int32(cr.Position),
			CurrentValue:            cr.CurrentValue,
			HasValue:                cr.HasValue,
			Satisfied:               cr.Satisfied,
			ProgressRatio:           cr.Ratio,
			ConsideredActivityCount: int32(cr.ConsideredCount),
			CountedActivityCount:    int32(cr.CountedCount),
		}
	}
	return out
}

// batchActivitiesByUser issues one ListActivitiesForUsersInRange call for
// every distinct user in goals, over the union of their periods, and groups
// the result by user id. This is the single shared batch-fetch behind both
// evaluateGoals (goal.go) and the timeline's per-goal running walks
// (feed.go), so neither can regress into one query per goal or per user.
//
// Judgment call: goals with widely different periods will make this one
// query span a wide date range (and pull activities no goal in the batch
// actually needs). That's accepted here in favor of simplicity — the union
// is still exactly one query regardless of how many goals or users are in
// the batch, and callers (progress.Evaluate, or the timeline's own
// sport/period filter) discard everything outside a given goal's own
// window, so correctness never depends on how tight the fetched range is.
func (d *Deps) batchActivitiesByUser(ctx context.Context, goals []db.Goal) (map[string][]metrics.Activity, error) {
	if len(goals) == 0 {
		return map[string][]metrics.Activity{}, nil
	}

	userSet := make(map[string]struct{}, len(goals))
	minStart := goals[0].PeriodStart
	maxEnd := goals[0].PeriodEnd
	for _, g := range goals {
		userSet[g.UserID] = struct{}{}
		if g.PeriodStart < minStart {
			minStart = g.PeriodStart
		}
		if g.PeriodEnd > maxEnd {
			maxEnd = g.PeriodEnd
		}
	}
	userIDs := make([]string, 0, len(userSet))
	for u := range userSet {
		userIDs = append(userIDs, u)
	}

	rows, err := d.Queries.ListActivitiesForUsersInRange(ctx, db.ListActivitiesForUsersInRangeParams{
		UserIds:     userIDs,
		StartDate:   minStart,
		StartDate_2: maxEnd,
	})
	if err != nil {
		return nil, err
	}

	actsByUser := make(map[string][]metrics.Activity, len(userSet))
	for _, r := range rows {
		a := metrics.FromDB(r)
		actsByUser[a.UserID] = append(actsByUser[a.UserID], a)
	}
	return actsByUser, nil
}

// evaluateGoals batch-evaluates every goal in goals against its own clauses
// (clausesByGoalID), using batchActivitiesByUser for the shared one-query
// activity fetch. progress.Evaluate does its own per-goal sport/period
// filtering, so the batch query only needs to be a safe superset — this is
// what keeps ListGoals and the leaderboard from regressing into the old N+1
// pattern.
func (d *Deps) evaluateGoals(ctx context.Context, goals []db.Goal, clausesByGoalID map[string][]progress.Clause) (map[string]progress.Result, error) {
	results := make(map[string]progress.Result, len(goals))
	if len(goals) == 0 {
		return results, nil
	}

	actsByUser, err := d.batchActivitiesByUser(ctx, goals)
	if err != nil {
		return nil, err
	}

	for _, g := range goals {
		pg := progress.Goal{
			ID:          g.ID,
			Sport:       sports.Selector{Group: sports.Group(g.SportGroup.String), SportType: g.SportType.String},
			PeriodStart: g.PeriodStart,
			PeriodEnd:   g.PeriodEnd,
			Clauses:     clausesByGoalID[g.ID],
		}
		results[g.ID] = progress.Evaluate(pg, actsByUser[g.UserID])
	}
	return results, nil
}

// clauseDefsToProto maps a goal's persisted clause definitions
// (scope/metric/comparator/value) to their wire GoalClause form — shared by
// Goal.clauses and LeaderboardEntry.clause_defs, so a leaderboard entry can
// label another member's clauses without ever loading that member's own
// goals (see LeaderboardEntry.clause_defs' doc comment in feed.proto).
func clauseDefsToProto(clauses []db.GoalClause) []*fitterv1.GoalClause {
	out := make([]*fitterv1.GoalClause, len(clauses))
	for i, c := range clauses {
		out[i] = &fitterv1.GoalClause{
			Id:         c.ID,
			Position:   int32(c.Position),
			Scope:      scopeFromDB(c.Scope),
			Metric:     keyToMetric(metrics.Key(c.Metric)),
			Comparator: comparatorFromDB(c.Comparator),
			Value:      c.Value,
		}
	}
	return out
}

// toGoalProto maps a persisted goal, its clauses, and its evaluated result
// to the wire Goal message.
func toGoalProto(g db.Goal, clauses []db.GoalClause, res progress.Result) *fitterv1.Goal {
	protoClauses := clauseDefsToProto(clauses)

	progressProto := &fitterv1.GoalProgress{
		Satisfied:               res.Satisfied,
		ProgressRatio:           res.Ratio,
		Clauses:                 clauseResultsToProto(res.Clauses),
		ConsideredActivityCount: int32(res.ConsideredCount),
	}
	if res.QualifyingActivityID != 0 {
		v := res.QualifyingActivityID
		progressProto.QualifyingActivityId = &v
	}
	if res.BestAttemptActivityID != 0 {
		v := res.BestAttemptActivityID
		progressProto.BestAttemptActivityId = &v
	}

	var campaignID *string
	if g.CampaignID.Valid {
		v := g.CampaignID.String
		campaignID = &v
	}

	return &fitterv1.Goal{
		Id:          g.ID,
		CampaignId:  campaignID,
		Name:        g.Name,
		Sport:       sportSelectorFromDB(g.SportGroup, g.SportType),
		PeriodStart: unixToTimestamp(g.PeriodStart),
		PeriodEnd:   unixToTimestamp(g.PeriodEnd),
		Clauses:     protoClauses,
		Progress:    progressProto,
		CreatedAt:   unixToTimestamp(g.CreatedAt),
	}
}

// toGoalProtosWithProgress batch-evaluates goals (one activity query, see
// evaluateGoals) and maps each to its wire Goal. It must only be called
// after any transaction the caller used to write these goals has already
// been committed: evaluateGoals reads through d.Queries (the ambient,
// non-transactional connection), and in tests the underlying *sql.DB is
// pinned to a single connection (dbtest), so calling this while the
// caller's own write transaction is still open would deadlock against it.
func (d *Deps) toGoalProtosWithProgress(ctx context.Context, goals []db.Goal, clausesByGoal map[string][]db.GoalClause) ([]*fitterv1.Goal, error) {
	if len(goals) == 0 {
		return nil, nil
	}
	progressClauses := make(map[string][]progress.Clause, len(goals))
	for gid, cs := range clausesByGoal {
		pc, err := clausesToProgress(cs)
		if err != nil {
			return nil, err
		}
		progressClauses[gid] = pc
	}
	results, err := d.evaluateGoals(ctx, goals, progressClauses)
	if err != nil {
		return nil, err
	}
	out := make([]*fitterv1.Goal, len(goals))
	for i, g := range goals {
		out[i] = toGoalProto(g, clausesByGoal[g.ID], results[g.ID])
	}
	return out, nil
}

// createGoalInTx validates msg and inserts one goal plus its clauses using
// qtx, which must already be bound to an open transaction
// (Queries.WithTx). It is the single creation path shared by
// GoalService.CreateGoal and CampaignService.JoinCampaign (via
// joinCampaignTx), so goal-creation validation and the Global Constraints
// period rule can never diverge between the two callers.
//
// When campaignOverride is non-nil (the JoinCampaign path, and
// CreateCampaign's creator auto-join with no goals), campaign_id and the
// period are forced to that campaign regardless of what msg carries. When
// nil, a campaign_id in msg is honored only after confirming userID is a
// member of it, and the period is still forced to that campaign's own
// window (Global Constraints) rather than trusting the request.
func (d *Deps) createGoalInTx(ctx context.Context, qtx *db.Queries, userID string, msg *fitterv1.CreateGoalRequest, campaignOverride *db.Campaign) (db.Goal, []db.GoalClause, error) {
	if msg.GetName() == "" {
		return db.Goal{}, nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	prepared, err := validateClauses(msg.GetClauses())
	if err != nil {
		return db.Goal{}, nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	sportGroup, sportType, err := sportSelectorToDB(msg.GetSport())
	if err != nil {
		return db.Goal{}, nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	campaignID := sql.NullString{}
	periodStart := timestampToUnix(msg.GetPeriodStart())
	periodEnd := timestampToUnix(msg.GetPeriodEnd())

	switch {
	case campaignOverride != nil:
		campaignID = sql.NullString{String: campaignOverride.ID, Valid: true}
		periodStart = campaignOverride.StartDate
		periodEnd = campaignOverride.EndDate
	case msg.GetCampaignId() != "":
		cid := msg.GetCampaignId()
		isMember, err := qtx.IsCampaignMember(ctx, db.IsCampaignMemberParams{CampaignID: cid, UserID: userID})
		if err != nil {
			return db.Goal{}, nil, connect.NewError(connect.CodeInternal, err)
		}
		if !isMember {
			return db.Goal{}, nil, connect.NewError(connect.CodePermissionDenied, errors.New("must join campaign to add a goal to it"))
		}
		c, err := qtx.GetCampaign(ctx, cid)
		if errors.Is(err, sql.ErrNoRows) {
			return db.Goal{}, nil, connect.NewError(connect.CodeNotFound, errors.New("campaign not found"))
		}
		if err != nil {
			return db.Goal{}, nil, connect.NewError(connect.CodeInternal, err)
		}
		campaignID = sql.NullString{String: cid, Valid: true}
		periodStart = c.StartDate
		periodEnd = c.EndDate
	}

	if periodEnd <= periodStart {
		return db.Goal{}, nil, connect.NewError(connect.CodeInvalidArgument, errors.New("period_end must be after period_start"))
	}

	g, err := qtx.CreateGoal(ctx, db.CreateGoalParams{
		ID:          uuid.NewString(),
		UserID:      userID,
		CampaignID:  campaignID,
		Name:        msg.GetName(),
		SportGroup:  sportGroup,
		SportType:   sportType,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		return db.Goal{}, nil, connect.NewError(connect.CodeInternal, err)
	}

	clauses := make([]db.GoalClause, len(prepared))
	for i, p := range prepared {
		gc, err := qtx.CreateGoalClause(ctx, db.CreateGoalClauseParams{
			ID:         uuid.NewString(),
			GoalID:     g.ID,
			Position:   int64(i),
			Scope:      p.scope,
			Metric:     string(p.metricKey),
			Comparator: p.comparator,
			Value:      p.value,
		})
		if err != nil {
			return db.Goal{}, nil, connect.NewError(connect.CodeInternal, err)
		}
		clauses[i] = gc
	}

	return g, clauses, nil
}

func (s *GoalService) CreateGoal(ctx context.Context, req *connect.Request[fitterv1.CreateGoalRequest]) (*connect.Response[fitterv1.CreateGoalResponse], error) {
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

	g, clauses, err := s.createGoalInTx(ctx, qtx, userID, req.Msg, nil)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoGoals, err := s.toGoalProtosWithProgress(ctx, []db.Goal{g}, map[string][]db.GoalClause{g.ID: clauses})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&fitterv1.CreateGoalResponse{Goal: protoGoals[0]}), nil
}

func (s *GoalService) ListGoals(ctx context.Context, req *connect.Request[fitterv1.ListGoalsRequest]) (*connect.Response[fitterv1.ListGoalsResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}

	var goals []db.Goal
	if cid := req.Msg.GetCampaignId(); cid != "" {
		isMember, err := s.Queries.IsCampaignMember(ctx, db.IsCampaignMemberParams{CampaignID: cid, UserID: userID})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !isMember {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("must join campaign to view its goals"))
		}
		goals, err = s.Queries.ListGoalsForUserInCampaign(ctx, db.ListGoalsForUserInCampaignParams{
			UserID:     userID,
			CampaignID: sql.NullString{String: cid, Valid: true},
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		var err error
		goals, err = s.Queries.ListPersonalGoalsForUser(ctx, userID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if len(goals) == 0 {
		return connect.NewResponse(&fitterv1.ListGoalsResponse{Goals: []*fitterv1.Goal{}}), nil
	}

	goalIDs := make([]string, len(goals))
	for i, g := range goals {
		goalIDs[i] = g.ID
	}
	clauseRows, err := s.Queries.ListGoalClausesForGoals(ctx, goalIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	clausesByGoal := make(map[string][]db.GoalClause, len(goals))
	for _, c := range clauseRows {
		clausesByGoal[c.GoalID] = append(clausesByGoal[c.GoalID], c)
	}

	out, err := s.toGoalProtosWithProgress(ctx, goals, clausesByGoal)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.ListGoalsResponse{Goals: out}), nil
}

func (s *GoalService) UpdateGoal(ctx context.Context, req *connect.Request[fitterv1.UpdateGoalRequest]) (*connect.Response[fitterv1.UpdateGoalResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	msg := req.Msg

	prepared, err := validateClauses(msg.GetClauses())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	sportGroup, sportType, err := sportSelectorToDB(msg.GetSport())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
	qtx := s.Queries.WithTx(tx)

	// Ownership: a goal that exists but belongs to another user reads as
	// NotFound, never as a distinct "forbidden" — same as UpdateGoalMeta's
	// WHERE id = ? AND user_id = ? would report it, which we keep below as
	// defense in depth.
	existing, err := qtx.GetGoal(ctx, msg.GetGoalId())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("goal not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if existing.UserID != userID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("goal not found"))
	}

	// Global Constraints: a campaign goal's period always matches its
	// campaign's window, so an update can't drift it away — the request's
	// period is honored only for a personal (non-campaign) goal.
	periodStart := timestampToUnix(msg.GetPeriodStart())
	periodEnd := timestampToUnix(msg.GetPeriodEnd())
	if existing.CampaignID.Valid {
		c, err := qtx.GetCampaign(ctx, existing.CampaignID.String)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		periodStart = c.StartDate
		periodEnd = c.EndDate
	}
	if periodEnd <= periodStart {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("period_end must be after period_start"))
	}

	g, err := qtx.UpdateGoalMeta(ctx, db.UpdateGoalMetaParams{
		Name:        msg.GetName(),
		SportGroup:  sportGroup,
		SportType:   sportType,
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

	if err := qtx.DeleteGoalClausesForGoal(ctx, g.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	clauses := make([]db.GoalClause, len(prepared))
	for i, p := range prepared {
		gc, err := qtx.CreateGoalClause(ctx, db.CreateGoalClauseParams{
			ID:         uuid.NewString(),
			GoalID:     g.ID,
			Position:   int64(i),
			Scope:      p.scope,
			Metric:     string(p.metricKey),
			Comparator: p.comparator,
			Value:      p.value,
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		clauses[i] = gc
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoGoals, err := s.toGoalProtosWithProgress(ctx, []db.Goal{g}, map[string][]db.GoalClause{g.ID: clauses})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&fitterv1.UpdateGoalResponse{Goal: protoGoals[0]}), nil
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
