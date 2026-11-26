package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	netmail "net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/mail"
)

const (
	codeTTL         = 15 * time.Minute
	codeCooldown    = time.Minute
	codeMaxTries    = 5
	feedbackMaxRune = 4000
)

// feedbackKinds maps the request's kind to the word used in the email subject.
var feedbackKinds = map[string]string{"bug": "Bug", "idea": "Idea", "other": "Feedback"}

// SendEmailCode emails a 6-digit code to the address the caller wants to verify. A new request
// replaces the previous code, at most once a minute.
// shortcut: no daily cap per member (members are admin-approved); add one here if it's abused.
func (s *UserService) SendEmailCode(ctx context.Context, req *connect.Request[fitterv1.SendEmailCodeRequest]) (*connect.Response[fitterv1.SendEmailCodeResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	email := strings.TrimSpace(req.Msg.GetEmail())
	if a, err := netmail.ParseAddress(email); err != nil || a.Address != email || len(email) > 254 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email must be a bare address"))
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	code := fmt.Sprintf("%06d", n.Int64())
	now := time.Now()
	_, err = s.Queries.ClaimEmailVerification(ctx, db.ClaimEmailVerificationParams{
		UserID: userID, Email: email, CodeHash: s.codeHash(userID, code),
		SentAt: now.Unix(), ExpiresAt: now.Add(codeTTL).Unix(), SentBefore: now.Add(-codeCooldown).Unix(),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("a code was sent less than a minute ago"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = s.Mail(ctx, mail.Message{
		To:      email,
		Subject: "Your Kombeweeg code: " + code,
		Body:    "Your Kombeweeg verification code is " + code + ".\n\nIt works for 15 minutes. If you didn't ask for it, ignore this email.\n",
	})
	if err != nil {
		slog.Error("send verification code", "user", userID, "error", err)
		// Drop the unsent code so the member can ask again straight away.
		if derr := s.Queries.DeleteEmailVerification(ctx, userID); derr != nil {
			slog.Error("drop unsent verification code", "user", userID, "error", derr)
		}
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("the code couldn't be sent"))
	}
	return connect.NewResponse(&fitterv1.SendEmailCodeResponse{}), nil
}

// VerifyEmail checks a code from SendEmailCode. Each check uses up one of five tries, counted
// before comparing so parallel guesses can't skip the count; a match saves the address.
func (s *UserService) VerifyEmail(ctx context.Context, req *connect.Request[fitterv1.VerifyEmailRequest]) (*connect.Response[fitterv1.VerifyEmailResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	v, err := s.Queries.CountEmailVerificationAttempt(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no code was requested"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if time.Now().Unix() > v.ExpiresAt {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the code has expired"))
	}
	if v.Attempts > codeMaxTries {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("too many tries; request a new code"))
	}
	if !hmac.Equal([]byte(v.CodeHash), []byte(s.codeHash(userID, strings.TrimSpace(req.Msg.GetCode())))) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the code doesn't match"))
	}
	u, err := s.Queries.SetUserEmail(ctx, db.SetUserEmailParams{ID: userID, Email: v.Email})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := s.Queries.DeleteEmailVerification(ctx, userID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&fitterv1.VerifyEmailResponse{User: currentUserToProto(u)}), nil
}

// SubmitFeedback emails the caller's feedback to the developer, with Reply-To set to the caller's
// verified address so a reply reaches them.
func (s *UserService) SubmitFeedback(ctx context.Context, req *connect.Request[fitterv1.SubmitFeedbackRequest]) (*connect.Response[fitterv1.SubmitFeedbackResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user"))
	}
	kind, ok := feedbackKinds[req.Msg.GetKind()]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kind must be bug, idea or other"))
	}
	message := strings.TrimSpace(req.Msg.GetMessage())
	if n := utf8.RuneCountInString(message); n < 1 || n > feedbackMaxRune {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("message must be 1 to %d characters", feedbackMaxRune))
	}
	release := req.Msg.GetRelease()
	if len(release) > 32 {
		release = release[:32]
	}
	u, err := s.Queries.GetUserByID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if u.Email == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("verify an email first"))
	}
	env := s.AppEnv
	if env == "" {
		env = "local"
	}
	err = s.Mail(ctx, mail.Message{
		To:      s.FeedbackTo,
		ReplyTo: u.Email,
		Subject: fmt.Sprintf("[Kombeweeg %s] %s from %s", env, kind, u.Name),
		Body: fmt.Sprintf("%s\n\n--\nFrom: %s (user %s)\nReply to: %s\nRelease: %s\nEnvironment: %s\n",
			message, u.Name, u.ID, u.Email, release, env),
	})
	if err != nil {
		slog.Error("send feedback", "user", userID, "error", err)
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("the feedback couldn't be sent"))
	}
	return connect.NewResponse(&fitterv1.SubmitFeedbackResponse{}), nil
}

// codeHash keys the code by the JWT secret and the member, so a leaked row can't be brute-forced
// offline and a code can't be replayed against another member's row.
func (s *UserService) codeHash(userID, code string) string {
	m := hmac.New(sha256.New, s.JWTSecret)
	m.Write([]byte(userID + ":" + code))
	return hex.EncodeToString(m.Sum(nil))
}
