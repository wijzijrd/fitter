package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/gen/fitter/v1/fitterv1connect"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/dbtest"
	"github.com/fitter/svc/internal/service"
)

var secret = []byte("test-secret")

func setup(t *testing.T) (fitterv1connect.UserServiceClient, fitterv1connect.CampaignServiceClient, *db.Queries, func(id string) string, func()) {
	t.Helper()
	sqlDB, q := dbtest.New(t)
	deps := &service.Deps{DB: sqlDB, Queries: q}
	mux := http.NewServeMux()
	opt := connect.WithInterceptors(auth.NewInterceptor(secret, q))
	mux.Handle(fitterv1connect.NewUserServiceHandler(service.NewUserService(deps), opt))
	mux.Handle(fitterv1connect.NewCampaignServiceHandler(service.NewCampaignService(deps), opt))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	tok := func(id string) string {
		s, err := auth.Issue(secret, id, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	return fitterv1connect.NewUserServiceClient(srv.Client(), srv.URL),
		fitterv1connect.NewCampaignServiceClient(srv.Client(), srv.URL), q, tok, func() { sqlDB.Close() }
}

func withTok[T any](m *T, tok string) *connect.Request[T] {
	r := connect.NewRequest(m)
	r.Header().Set("Authorization", "Bearer "+tok)
	return r
}

func seed(t *testing.T, q *db.Queries, id string, n int64) {
	t.Helper()
	if _, err := q.UpsertUserByStravaID(context.Background(), db.UpsertUserByStravaIDParams{ID: id, StravaAthleteID: n, Name: id}); err != nil {
		t.Fatal(err)
	}
}

func TestInterceptorStatusGate(t *testing.T) {
	users, campaigns, q, tok, _ := setup(t)
	ctx := context.Background()
	seed(t, q, "pend", 1) // default status: pending

	if _, err := users.GetCurrentUser(ctx, withTok(&fitterv1.GetCurrentUserRequest{}, tok("pend"))); err != nil {
		t.Fatalf("pending GetCurrentUser: %v", err)
	}
	_, err := campaigns.ListCampaigns(ctx, withTok(&fitterv1.ListCampaignsRequest{}, tok("pend")))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("pending ListCampaigns = %v, want PermissionDenied", err)
	}

	if err := q.SetUserStatus(ctx, db.SetUserStatusParams{ID: "pend", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if _, err := campaigns.ListCampaigns(ctx, withTok(&fitterv1.ListCampaignsRequest{}, tok("pend"))); err != nil {
		t.Fatalf("active ListCampaigns: %v", err)
	}

	if err := q.SetUserStatus(ctx, db.SetUserStatusParams{ID: "pend", Status: "banned"}); err != nil {
		t.Fatal(err)
	}
	_, err = users.GetCurrentUser(ctx, withTok(&fitterv1.GetCurrentUserRequest{}, tok("pend")))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("banned GetCurrentUser = %v, want Unauthenticated", err)
	}
}

func TestInterceptorUnknownUserUnauthenticated(t *testing.T) {
	users, _, _, tok, _ := setup(t)
	_, err := users.GetCurrentUser(context.Background(), withTok(&fitterv1.GetCurrentUserRequest{}, tok("ghost")))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("unknown user = %v, want Unauthenticated", err)
	}
}

func TestInterceptorDBErrorIsUnavailable(t *testing.T) {
	users, _, q, tok, closeDB := setup(t)
	seed(t, q, "u", 1)
	closeDB()
	_, err := users.GetCurrentUser(context.Background(), withTok(&fitterv1.GetCurrentUserRequest{}, tok("u")))
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("db error = %v, want Unavailable", err)
	}
}
