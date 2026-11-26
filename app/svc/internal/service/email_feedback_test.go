package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/mail"
	"github.com/fitter/svc/internal/service"
)

// newMailDeps is newDeps plus a JWT secret and a Mail func that records what would be sent.
func newMailDeps(t *testing.T) (*service.Deps, *[]mail.Message) {
	t.Helper()
	d := newDeps(t)
	sent := &[]mail.Message{}
	d.JWTSecret = []byte("test-secret")
	d.FeedbackTo = "dev@example.com"
	d.AppEnv = "staging"
	d.Mail = func(_ context.Context, m mail.Message) error { *sent = append(*sent, m); return nil }
	return d, sent
}

func wantCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	if got := connect.CodeOf(err); err == nil || got != code {
		t.Fatalf("got %v (%v), want %v", err, got, code)
	}
}

func sendCode(t *testing.T, svc *service.UserService, ctx context.Context, sent *[]mail.Message, email string) string {
	t.Helper()
	if _, err := svc.SendEmailCode(ctx, connect.NewRequest(&fitterv1.SendEmailCodeRequest{Email: email})); err != nil {
		t.Fatalf("SendEmailCode: %v", err)
	}
	m := (*sent)[len(*sent)-1]
	if m.To != email {
		t.Fatalf("code sent to %q, want %q", m.To, email)
	}
	return m.Subject[len(m.Subject)-6:]
}

func verify(svc *service.UserService, ctx context.Context, code string) (*connect.Response[fitterv1.VerifyEmailResponse], error) {
	return svc.VerifyEmail(ctx, connect.NewRequest(&fitterv1.VerifyEmailRequest{Code: code}))
}

func TestEmailVerification_HappyPath(t *testing.T) {
	d, sent := newMailDeps(t)
	sam := seedUser(t, d.Queries, "sam")
	svc, ctx := service.NewUserService(d), ctxAs(sam.ID)

	code := sendCode(t, svc, ctx, sent, "sam@example.com")
	resp, err := verify(svc, ctx, code)
	if err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if resp.Msg.User.Email != "sam@example.com" {
		t.Fatalf("email = %q", resp.Msg.User.Email)
	}
	_, err = verify(svc, ctx, code) // the code is used up
	wantCode(t, err, connect.CodeFailedPrecondition)
}

func TestEmailVerification_RejectsBadAddresses(t *testing.T) {
	d, _ := newMailDeps(t)
	sam := seedUser(t, d.Queries, "sam")
	svc, ctx := service.NewUserService(d), ctxAs(sam.ID)
	for _, email := range []string{"", "not-an-email", "Sam <sam@example.com>", strings.Repeat("a", 250) + "@x.io"} {
		_, err := svc.SendEmailCode(ctx, connect.NewRequest(&fitterv1.SendEmailCodeRequest{Email: email}))
		wantCode(t, err, connect.CodeInvalidArgument)
	}
}

func TestEmailVerification_FiveTriesThenRefused(t *testing.T) {
	d, sent := newMailDeps(t)
	sam := seedUser(t, d.Queries, "sam")
	svc, ctx := service.NewUserService(d), ctxAs(sam.ID)

	code := sendCode(t, svc, ctx, sent, "sam@example.com")
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for i := 0; i < 5; i++ {
		_, err := verify(svc, ctx, wrong)
		wantCode(t, err, connect.CodeInvalidArgument)
	}
	_, err := verify(svc, ctx, code) // right code, but the tries are spent
	wantCode(t, err, connect.CodeFailedPrecondition)
}

func TestEmailVerification_ExpiredCode(t *testing.T) {
	d, sent := newMailDeps(t)
	sam := seedUser(t, d.Queries, "sam")
	svc, ctx := service.NewUserService(d), ctxAs(sam.ID)

	code := sendCode(t, svc, ctx, sent, "sam@example.com")
	if _, err := d.DB.Exec(`UPDATE email_verifications SET expires_at = 1`); err != nil {
		t.Fatal(err)
	}
	_, err := verify(svc, ctx, code)
	wantCode(t, err, connect.CodeFailedPrecondition)
}

func TestEmailVerification_ResendCooldown(t *testing.T) {
	d, sent := newMailDeps(t)
	sam := seedUser(t, d.Queries, "sam")
	svc, ctx := service.NewUserService(d), ctxAs(sam.ID)

	sendCode(t, svc, ctx, sent, "sam@example.com")
	_, err := svc.SendEmailCode(ctx, connect.NewRequest(&fitterv1.SendEmailCodeRequest{Email: "sam@example.com"}))
	wantCode(t, err, connect.CodeResourceExhausted)

	if _, err := d.DB.Exec(`UPDATE email_verifications SET sent_at = sent_at - 61`); err != nil {
		t.Fatal(err)
	}
	code := sendCode(t, svc, ctx, sent, "sam@other.example") // a resend replaces the code and address
	resp, err := verify(svc, ctx, code)
	if err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if resp.Msg.User.Email != "sam@other.example" {
		t.Fatalf("email = %q", resp.Msg.User.Email)
	}
}

func TestEmailVerification_FailedSendCanRetryAtOnce(t *testing.T) {
	d, sent := newMailDeps(t)
	sam := seedUser(t, d.Queries, "sam")
	svc, ctx := service.NewUserService(d), ctxAs(sam.ID)

	ok := d.Mail
	d.Mail = func(context.Context, mail.Message) error { return errors.New("ses down") }
	_, err := svc.SendEmailCode(ctx, connect.NewRequest(&fitterv1.SendEmailCodeRequest{Email: "sam@example.com"}))
	wantCode(t, err, connect.CodeUnavailable)
	d.Mail = ok
	sendCode(t, svc, ctx, sent, "sam@example.com") // no cooldown after a failed send
}

func TestSubmitFeedback(t *testing.T) {
	d, sent := newMailDeps(t)
	sam := seedUser(t, d.Queries, "sam")
	svc, ctx := service.NewUserService(d), ctxAs(sam.ID)
	submit := func(kind, msg string) error {
		_, err := svc.SubmitFeedback(ctx, connect.NewRequest(&fitterv1.SubmitFeedbackRequest{Kind: kind, Message: msg, Release: "2026-11-22"}))
		return err
	}

	wantCode(t, submit("idea", "More charts"), connect.CodeFailedPrecondition) // not verified yet

	if _, err := verify(svc, ctx, sendCode(t, svc, ctx, sent, "sam@example.com")); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	wantCode(t, submit("rant", "More charts"), connect.CodeInvalidArgument)
	wantCode(t, submit("idea", "   "), connect.CodeInvalidArgument)
	wantCode(t, submit("idea", strings.Repeat("é", 4001)), connect.CodeInvalidArgument)

	if err := submit("idea", strings.Repeat("é", 4000)); err != nil {
		t.Fatalf("4000 characters should fit: %v", err)
	}
	if err := submit("bug", "  The feed is blank  "); err != nil {
		t.Fatalf("SubmitFeedback: %v", err)
	}
	m := (*sent)[len(*sent)-1]
	if m.To != "dev@example.com" || m.ReplyTo != "sam@example.com" || m.Subject != "[Kombeweeg staging] Bug from sam" {
		t.Fatalf("mail = %+v", m)
	}
	if !strings.HasPrefix(m.Body, "The feed is blank\n") || !strings.Contains(m.Body, "Release: 2026-11-22") {
		t.Fatalf("body = %q", m.Body)
	}
}
