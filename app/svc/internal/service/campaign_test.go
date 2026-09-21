package service_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/service"
)

func baseCreateCampaignRequest(start, end int64) *fitterv1.CreateCampaignRequest {
	return &fitterv1.CreateCampaignRequest{
		Name:        "Fall challenge",
		Description: "A fall fitness challenge",
		StartDate:   ts(start),
		EndDate:     ts(end),
	}
}

func TestCreateCampaign_MetricTargetMustBeBothOrNeither(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	svc := service.NewCampaignService(deps)
	ctx := ctxAs(alice.ID)

	start := time.Now().Unix()
	end := time.Now().Add(30 * 24 * time.Hour).Unix()

	req := baseCreateCampaignRequest(start, end)
	metric := fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE
	req.Metric = &metric
	// TargetValue deliberately left unset.

	_, err := svc.CreateCampaign(ctx, connect.NewRequest(req))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateCampaign(metric set, target unset) error = %v, want CodeInvalidArgument", err)
	}

	target := 100000.0
	req2 := baseCreateCampaignRequest(start, end)
	req2.TargetValue = &target
	// Metric deliberately left unset.
	_, err = svc.CreateCampaign(ctx, connect.NewRequest(req2))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateCampaign(target set, metric unset) error = %v, want CodeInvalidArgument", err)
	}
}

func TestCreateCampaign_EndDateMustBeAfterStartDate(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	svc := service.NewCampaignService(deps)
	ctx := ctxAs(alice.ID)

	start := time.Now().Unix()
	req := baseCreateCampaignRequest(start, start) // end == start
	_, err := svc.CreateCampaign(ctx, connect.NewRequest(req))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateCampaign(end_date == start_date) error = %v, want CodeInvalidArgument", err)
	}

	req2 := baseCreateCampaignRequest(start, start-3600) // end before start
	_, err = svc.CreateCampaign(ctx, connect.NewRequest(req2))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateCampaign(end_date < start_date) error = %v, want CodeInvalidArgument", err)
	}
}

func TestJoinCampaign_CreatesGoalsWithForcedCampaignAndPeriod(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	bob := seedUser(t, deps.Queries, "bob")
	campaignSvc := service.NewCampaignService(deps)

	start := time.Now().Add(-24 * time.Hour).Unix()
	end := time.Now().Add(30 * 24 * time.Hour).Unix()
	createResp, err := campaignSvc.CreateCampaign(ctxAs(alice.ID), connect.NewRequest(baseCreateCampaignRequest(start, end)))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	campaignID := createResp.Msg.GetCampaign().GetId()

	// bob's goals deliberately carry a bogus campaign_id and an unrelated
	// period — both must be overwritten server-side.
	wrongPeriodStart := ts(0)
	wrongPeriodEnd := ts(1)
	bogusCampaignID := "not-a-real-campaign"
	joinReq := &fitterv1.JoinCampaignRequest{
		CampaignId: campaignID,
		Goals: []*fitterv1.CreateGoalRequest{
			{
				CampaignId:  &bogusCampaignID,
				Name:        "bob goal 1",
				PeriodStart: wrongPeriodStart,
				PeriodEnd:   wrongPeriodEnd,
				Clauses: []*fitterv1.GoalClause{{
					Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE,
					Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
					Comparator: fitterv1.Comparator_COMPARATOR_GTE,
					Value:      1000,
				}},
			},
			{
				CampaignId:  &bogusCampaignID,
				Name:        "bob goal 2",
				PeriodStart: wrongPeriodStart,
				PeriodEnd:   wrongPeriodEnd,
				Clauses: []*fitterv1.GoalClause{{
					Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_MOVING_TIME,
					Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
					Comparator: fitterv1.Comparator_COMPARATOR_GTE,
					Value:      500,
				}},
			},
		},
	}

	joinResp, err := campaignSvc.JoinCampaign(ctxAs(bob.ID), connect.NewRequest(joinReq))
	if err != nil {
		t.Fatalf("JoinCampaign: %v", err)
	}
	if len(joinResp.Msg.GetGoals()) != 2 {
		t.Fatalf("JoinCampaign: got %d goals, want 2", len(joinResp.Msg.GetGoals()))
	}
	for _, g := range joinResp.Msg.GetGoals() {
		if g.GetCampaignId() != campaignID {
			t.Errorf("goal %q campaign_id = %q, want %q", g.GetName(), g.GetCampaignId(), campaignID)
		}
		if g.GetPeriodStart().AsTime().Unix() != start || g.GetPeriodEnd().AsTime().Unix() != end {
			t.Errorf("goal %q period = [%v, %v], want campaign window [%v, %v]",
				g.GetName(), g.GetPeriodStart().AsTime().Unix(), g.GetPeriodEnd().AsTime().Unix(), start, end)
		}
	}

	// Confirm directly against the DB too, not just the response.
	rows, err := deps.Queries.ListGoalsForUserInCampaign(context.Background(), db.ListGoalsForUserInCampaignParams{
		UserID:     bob.ID,
		CampaignID: newValidNullString(campaignID),
	})
	if err != nil {
		t.Fatalf("ListGoalsForUserInCampaign: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("ListGoalsForUserInCampaign: got %d rows, want 2", len(rows))
	}
	for _, r := range rows {
		if !r.CampaignID.Valid || r.CampaignID.String != campaignID {
			t.Errorf("goal row %s campaign_id = %+v, want %q", r.ID, r.CampaignID, campaignID)
		}
		if r.PeriodStart != start || r.PeriodEnd != end {
			t.Errorf("goal row %s period = [%d, %d], want [%d, %d]", r.ID, r.PeriodStart, r.PeriodEnd, start, end)
		}
	}
}

func TestJoinCampaign_FailureMidTransactionRollsBackMembership(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	bob := seedUser(t, deps.Queries, "bob")
	campaignSvc := service.NewCampaignService(deps)

	start := time.Now().Add(-24 * time.Hour).Unix()
	end := time.Now().Add(30 * 24 * time.Hour).Unix()
	createResp, err := campaignSvc.CreateCampaign(ctxAs(alice.ID), connect.NewRequest(baseCreateCampaignRequest(start, end)))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	campaignID := createResp.Msg.GetCampaign().GetId()

	// First goal is valid; second has zero clauses, which createGoalInTx
	// rejects. The whole transaction — including the membership upsert —
	// must roll back.
	joinReq := &fitterv1.JoinCampaignRequest{
		CampaignId: campaignID,
		Goals: []*fitterv1.CreateGoalRequest{
			{
				Name: "valid goal",
				Clauses: []*fitterv1.GoalClause{{
					Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE,
					Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
					Comparator: fitterv1.Comparator_COMPARATOR_GTE,
					Value:      1000,
				}},
			},
			{
				Name:    "invalid goal, no clauses",
				Clauses: nil,
			},
		},
	}

	_, err = campaignSvc.JoinCampaign(ctxAs(bob.ID), connect.NewRequest(joinReq))
	if err == nil {
		t.Fatal("JoinCampaign with an invalid goal succeeded, want an error")
	}

	isMember, err := deps.Queries.IsCampaignMember(context.Background(), db.IsCampaignMemberParams{
		CampaignID: campaignID,
		UserID:     bob.ID,
	})
	if err != nil {
		t.Fatalf("IsCampaignMember: %v", err)
	}
	if isMember {
		t.Fatal("bob is a campaign member after a failed JoinCampaign — transaction did not roll back")
	}

	goals, err := deps.Queries.ListGoalsForUserInCampaign(context.Background(), db.ListGoalsForUserInCampaignParams{
		UserID:     bob.ID,
		CampaignID: newValidNullString(campaignID),
	})
	if err != nil {
		t.Fatalf("ListGoalsForUserInCampaign: %v", err)
	}
	if len(goals) != 0 {
		t.Fatalf("bob has %d goals in the campaign after a failed JoinCampaign, want 0", len(goals))
	}
}

func TestLeaveCampaign_DeletesOnlyThatUsersGoals(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	bob := seedUser(t, deps.Queries, "bob")
	campaignSvc := service.NewCampaignService(deps)

	start := time.Now().Add(-24 * time.Hour).Unix()
	end := time.Now().Add(30 * 24 * time.Hour).Unix()
	createResp, err := campaignSvc.CreateCampaign(ctxAs(alice.ID), connect.NewRequest(baseCreateCampaignRequest(start, end)))
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	campaignID := createResp.Msg.GetCampaign().GetId()

	oneGoalReq := func(name string) *fitterv1.JoinCampaignRequest {
		return &fitterv1.JoinCampaignRequest{
			CampaignId: campaignID,
			Goals: []*fitterv1.CreateGoalRequest{{
				Name: name,
				Clauses: []*fitterv1.GoalClause{{
					Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_DISTANCE,
					Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
					Comparator: fitterv1.Comparator_COMPARATOR_GTE,
					Value:      1000,
				}},
			}},
		}
	}

	if _, err := campaignSvc.JoinCampaign(ctxAs(bob.ID), connect.NewRequest(oneGoalReq("bob goal"))); err != nil {
		t.Fatalf("JoinCampaign(bob): %v", err)
	}
	// alice already auto-joined via CreateCampaign; give her a campaign goal
	// too so we can confirm it survives bob leaving.
	goalSvc := service.NewGoalService(deps)
	aliceCampaignID := campaignID
	if _, err := goalSvc.CreateGoal(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateGoalRequest{
		CampaignId: &aliceCampaignID,
		Name:       "alice goal",
		Clauses: []*fitterv1.GoalClause{{
			Metric:     fitterv1.ActivityMetric_ACTIVITY_METRIC_MOVING_TIME,
			Scope:      fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
			Comparator: fitterv1.Comparator_COMPARATOR_GTE,
			Value:      500,
		}},
	})); err != nil {
		t.Fatalf("CreateGoal(alice): %v", err)
	}

	if _, err := campaignSvc.LeaveCampaign(ctxAs(bob.ID), connect.NewRequest(&fitterv1.LeaveCampaignRequest{CampaignId: campaignID})); err != nil {
		t.Fatalf("LeaveCampaign(bob): %v", err)
	}

	bobGoals, err := deps.Queries.ListGoalsForUserInCampaign(context.Background(), db.ListGoalsForUserInCampaignParams{
		UserID:     bob.ID,
		CampaignID: newValidNullString(campaignID),
	})
	if err != nil {
		t.Fatalf("ListGoalsForUserInCampaign(bob): %v", err)
	}
	if len(bobGoals) != 0 {
		t.Fatalf("bob has %d campaign goals after leaving, want 0", len(bobGoals))
	}

	aliceGoals, err := deps.Queries.ListGoalsForUserInCampaign(context.Background(), db.ListGoalsForUserInCampaignParams{
		UserID:     alice.ID,
		CampaignID: newValidNullString(campaignID),
	})
	if err != nil {
		t.Fatalf("ListGoalsForUserInCampaign(alice): %v", err)
	}
	if len(aliceGoals) != 1 {
		t.Fatalf("alice has %d campaign goals after bob left, want 1 (untouched)", len(aliceGoals))
	}

	isBobMember, err := deps.Queries.IsCampaignMember(context.Background(), db.IsCampaignMemberParams{CampaignID: campaignID, UserID: bob.ID})
	if err != nil {
		t.Fatalf("IsCampaignMember(bob): %v", err)
	}
	if isBobMember {
		t.Fatal("bob is still a campaign member after LeaveCampaign")
	}
}
