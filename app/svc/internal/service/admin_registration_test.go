package service_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/service"
)

func TestRegistration_AdminOnlyModeAndAllowlist(t *testing.T) {
	deps := newDeps(t)
	ctx := context.Background()
	admin := seedUser(t, deps.Queries, "admin")
	if err := deps.Queries.SetUserStatus(ctx, db.SetUserStatusParams{ID: admin.ID, Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := deps.Queries.SetUserRole(ctx, db.SetUserRoleParams{ID: admin.ID, Role: "admin"}); err != nil {
		t.Fatal(err)
	}
	svc := service.NewAdminService(deps)
	as := ctxAsAdmin(admin.ID)
	member := seedUser(t, deps.Queries, "member")

	// Non-admins are refused on every registration RPC.
	nonAdmin := ctxAs(member.ID)
	if _, err := svc.GetRegistration(nonAdmin, connect.NewRequest(&fitterv1.GetRegistrationRequest{})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("get = %v", err)
	}
	if _, err := svc.SetRegistrationMode(nonAdmin, connect.NewRequest(&fitterv1.SetRegistrationModeRequest{Mode: "invite_only"})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("set mode = %v", err)
	}
	if _, err := svc.AllowAthlete(nonAdmin, connect.NewRequest(&fitterv1.AllowAthleteRequest{StravaAthleteId: 42})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("allow = %v", err)
	}

	// Registration starts open; the mode accepts only the two known values.
	got, err := svc.GetRegistration(as, connect.NewRequest(&fitterv1.GetRegistrationRequest{}))
	if err != nil || got.Msg.GetMode() != "open" || len(got.Msg.GetAllowed()) != 0 {
		t.Fatalf("default = %v, %v", got, err)
	}
	if _, err := svc.SetRegistrationMode(as, connect.NewRequest(&fitterv1.SetRegistrationModeRequest{Mode: "closed"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("bogus mode = %v", err)
	}
	if _, err := svc.AllowAthlete(as, connect.NewRequest(&fitterv1.AllowAthleteRequest{StravaAthleteId: 0})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("zero athlete = %v", err)
	}

	if _, err := svc.SetRegistrationMode(as, connect.NewRequest(&fitterv1.SetRegistrationModeRequest{Mode: "invite_only"})); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AllowAthlete(as, connect.NewRequest(&fitterv1.AllowAthleteRequest{StravaAthleteId: 42})); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AllowAthlete(as, connect.NewRequest(&fitterv1.AllowAthleteRequest{StravaAthleteId: 42})); err != nil {
		t.Fatalf("re-allow should be a no-op, got %v", err)
	}
	got, _ = svc.GetRegistration(as, connect.NewRequest(&fitterv1.GetRegistrationRequest{}))
	if got.Msg.GetMode() != "invite_only" || len(got.Msg.GetAllowed()) != 1 || got.Msg.GetAllowed()[0].GetStravaAthleteId() != 42 {
		t.Fatalf("after allow = %v", got.Msg)
	}

	if _, err := svc.RevokeAthlete(as, connect.NewRequest(&fitterv1.RevokeAthleteRequest{StravaAthleteId: 42})); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.GetRegistration(as, connect.NewRequest(&fitterv1.GetRegistrationRequest{}))
	if len(got.Msg.GetAllowed()) != 0 {
		t.Fatalf("after revoke = %v", got.Msg.GetAllowed())
	}
}
