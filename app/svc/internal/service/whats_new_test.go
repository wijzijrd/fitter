package service_test

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/service"
)

func TestWhatsNewSeen_StartsEmptyStaysPerUserAndValidates(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	bob := seedUser(t, deps.Queries, "bob")
	svc := service.NewUserService(deps)
	get := func(id string) string {
		r, err := svc.GetCurrentUser(ctxAs(id), connect.NewRequest(&fitterv1.GetCurrentUserRequest{}))
		if err != nil {
			t.Fatalf("GetCurrentUser: %v", err)
		}
		return r.Msg.GetUser().GetWhatsNewSeen()
	}
	mark := func(id, rel string) error {
		_, err := svc.MarkWhatsNewSeen(ctxAs(id), connect.NewRequest(&fitterv1.MarkWhatsNewSeenRequest{Release: rel}))
		return err
	}

	if got := get(alice.ID); got != "" {
		t.Fatalf("new user whats_new_seen = %q, want empty", got)
	}
	if err := mark(alice.ID, "2026-11-22"); err != nil {
		t.Fatalf("MarkWhatsNewSeen: %v", err)
	}
	if got := get(alice.ID); got != "2026-11-22" {
		t.Fatalf("after mark = %q", got)
	}
	if got := get(bob.ID); got != "" {
		t.Fatalf("bob's marker moved when alice's did: %q", got)
	}
	for _, bad := range []string{"", strings.Repeat("x", 33)} {
		if err := mark(alice.ID, bad); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("release %q: code %v, want InvalidArgument", bad, connect.CodeOf(err))
		}
	}
}
