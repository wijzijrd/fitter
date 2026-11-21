package service_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/service"
)

func TestAdminUserLifecycleAndAudit(t *testing.T) {
	deps := newDeps(t)
	ctx := context.Background()
	admin := seedUser(t, deps.Queries, "admin")
	bob := seedUser(t, deps.Queries, "bob") // pending
	if err := deps.Queries.SetUserStatus(ctx, db.SetUserStatusParams{ID: admin.ID, Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := deps.Queries.SetUserRole(ctx, db.SetUserRoleParams{ID: admin.ID, Role: "admin"}); err != nil {
		t.Fatal(err)
	}
	svc := service.NewAdminService(deps)
	as := ctxAsAdmin(admin.ID)

	if _, err := svc.ApproveUser(as, connect.NewRequest(&fitterv1.ApproveUserRequest{Id: bob.ID})); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetUserRole(as, connect.NewRequest(&fitterv1.SetUserRoleRequest{Id: bob.ID, Role: "admin"})); err != nil {
		t.Fatal(err)
	}
	// admins can't be banned until demoted
	if _, err := svc.BanUser(as, connect.NewRequest(&fitterv1.BanUserRequest{Id: bob.ID, Reason: "x"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("ban admin = %v", err)
	}
	if _, err := svc.SetUserRole(as, connect.NewRequest(&fitterv1.SetUserRoleRequest{Id: bob.ID, Role: "member"})); err != nil {
		t.Fatal(err)
	}
	// reason required + capped
	for _, r := range []string{"", strings.Repeat("a", 501)} {
		if _, err := svc.BanUser(as, connect.NewRequest(&fitterv1.BanUserRequest{Id: bob.ID, Reason: r})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("ban reason %d chars = %v", len(r), err)
		}
	}
	if _, err := svc.BanUser(as, connect.NewRequest(&fitterv1.BanUserRequest{Id: bob.ID, Reason: "spam"})); err != nil {
		t.Fatal(err)
	}
	u, _ := deps.Queries.GetUserByID(ctx, bob.ID)
	if u.Status != "banned" || u.BanReason != "spam" || u.BannedBy.String != admin.ID || !u.BannedAt.Valid {
		t.Fatalf("ban fields = %+v", u)
	}
	if _, err := svc.UnbanUser(as, connect.NewRequest(&fitterv1.UnbanUserRequest{Id: bob.ID})); err != nil {
		t.Fatal(err)
	}
	u, _ = deps.Queries.GetUserByID(ctx, bob.ID)
	if u.Status != "active" || u.BanReason != "" || u.BannedBy.Valid {
		t.Fatalf("unban fields = %+v", u)
	}

	got, err := svc.ListAdminActions(as, connect.NewRequest(&fitterv1.ListAdminActionsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, a := range got.Msg.Actions {
		actions = append(actions, a.Action)
	}
	// Same-second actions have no stable order, so compare sorted.
	sort.Strings(actions)
	want := "approve,ban,demote,promote,unban"
	if strings.Join(actions, ",") != want {
		t.Fatalf("audit = %v, want %s", actions, want)
	}
}

func TestAdminLockoutGuards(t *testing.T) {
	deps := newDeps(t)
	ctx := context.Background()
	admin := seedUser(t, deps.Queries, "admin")
	_ = deps.Queries.SetUserStatus(ctx, db.SetUserStatusParams{ID: admin.ID, Status: "active"})
	_ = deps.Queries.SetUserRole(ctx, db.SetUserRoleParams{ID: admin.ID, Role: "admin"})
	svc := service.NewAdminService(deps)
	as := ctxAsAdmin(admin.ID)

	if _, err := svc.BanUser(as, connect.NewRequest(&fitterv1.BanUserRequest{Id: admin.ID, Reason: "x"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("self-ban = %v", err)
	}
	if _, err := svc.SetUserRole(as, connect.NewRequest(&fitterv1.SetUserRoleRequest{Id: admin.ID, Role: "member"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("self-demote = %v", err)
	}
	// a second admin cannot demote the only *other*... but last-admin guard: demote when count<=1
	other := seedUser(t, deps.Queries, "other")
	_ = deps.Queries.SetUserStatus(ctx, db.SetUserStatusParams{ID: other.ID, Status: "active"})
	_ = deps.Queries.SetUserRole(ctx, db.SetUserRoleParams{ID: other.ID, Role: "admin"})
	_ = deps.Queries.SetUserStatus(ctx, db.SetUserStatusParams{ID: admin.ID, Status: "pending"}) // only `other` is an active admin
	if _, err := svc.SetUserRole(ctxAsAdmin(admin.ID), connect.NewRequest(&fitterv1.SetUserRoleRequest{Id: other.ID, Role: "member"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("demote last active admin = %v", err)
	}
}

func TestAdminUserRPCs_NonAdminRejected(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	svc := service.NewAdminService(deps)
	if _, err := svc.ApproveUser(ctxAs(alice.ID), connect.NewRequest(&fitterv1.ApproveUserRequest{Id: alice.ID})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("approve = %v", err)
	}
	if _, err := svc.ListUsers(ctxAs(alice.ID), connect.NewRequest(&fitterv1.ListUsersRequest{})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("list = %v", err)
	}
}

func TestBannedMemberHiddenFromCampaign(t *testing.T) {
	deps := newDeps(t)
	ctx := context.Background()
	bob := seedUser(t, deps.Queries, "bob")
	c, err := deps.Queries.CreateCampaign(ctx, db.CreateCampaignParams{ID: "c1", Name: "c", StartDate: 1, EndDate: 2, CreatedBy: bob.ID})
	if err != nil {
		t.Fatal(err)
	}
	_ = deps.Queries.JoinCampaign(ctx, db.JoinCampaignParams{CampaignID: c.ID, UserID: bob.ID})
	n, _ := deps.Queries.CountCampaignMembers(ctx, c.ID)
	if n != 1 {
		t.Fatalf("count = %d", n)
	}
	_ = deps.Queries.BanUser(ctx, db.BanUserParams{ID: bob.ID, BanReason: "x"})
	n, _ = deps.Queries.CountCampaignMembers(ctx, c.ID)
	m, _ := deps.Queries.ListCampaignMembers(ctx, c.ID)
	if n != 0 || len(m) != 0 {
		t.Fatalf("banned visible: count=%d members=%d", n, len(m))
	}
	_ = deps.Queries.UnbanUser(ctx, bob.ID)
	n, _ = deps.Queries.CountCampaignMembers(ctx, c.ID)
	if n != 1 {
		t.Fatalf("after unban count = %d", n)
	}
}
