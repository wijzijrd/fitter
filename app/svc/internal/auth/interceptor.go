package auth

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/fitter/svc/internal/db"
)

// exemptProcedures lists fully-qualified RPC procedure names (Spec.Procedure
// — "/<package>.<Service>/<Method>") that skip the bearer-token check. Kept
// deliberately tiny: CatalogService's metric/sport metadata is pure,
// DB-free translation that the login-time onboarding UI needs to fetch
// before a user necessarily has a session token. Add new exemptions here
// only when a procedure has no user-specific state, rather than
// restructuring auth more broadly.
var exemptProcedures = map[string]bool{
	"/fitter.v1.CatalogService/ListMetrics": true,
	"/fitter.v1.CatalogService/ListSports":  true,
}

// NewInterceptor returns a Connect interceptor that requires a valid
// "Authorization: Bearer <jwt>" header on every unary and streaming call
// (except exemptProcedures), and injects the authenticated user id and
// current role into the request context.
func NewInterceptor(secret []byte, queries userGetter) connect.Interceptor {
	return &interceptor{secret: secret, queries: queries}
}

// pendingAllowedProcedure is the only call a not-yet-approved user may make,
// so the UI can learn their status and show the waiting screen.
const pendingAllowedProcedure = "/fitter.v1.UserService/GetCurrentUser"

// userGetter is the one query the interceptor needs; *db.Queries satisfies it.
type userGetter interface {
	GetUserByID(ctx context.Context, id string) (db.User, error)
}

type interceptor struct {
	secret  []byte
	queries userGetter
}

func (i *interceptor) authenticate(ctx context.Context, procedure string, header interface{ Get(string) string }) (context.Context, error) {
	raw := header.Get("Authorization")
	token, ok := strings.CutPrefix(raw, "Bearer ")
	if !ok || token == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, ErrInvalidToken)
	}
	userID, err := Verify(i.secret, token)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	// Role is looked up fresh from the DB on every request rather than
	// carried in the JWT, so a role change (promotion or, in the future,
	// demotion) takes effect on the very next call instead of waiting for
	// the token to expire and be reissued.
	u, err := i.queries.GetUserByID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeUnauthenticated, ErrInvalidToken)
	}
	if err != nil {
		// A transient DB failure must not read as "logged out": the UI wipes
		// its JWT on Unauthenticated.
		slog.Error("auth user lookup failed", "user_id", userID, "err", err)
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("user lookup unavailable"))
	}
	switch u.Status {
	case "banned":
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("account banned"))
	case "pending":
		if procedure != pendingAllowedProcedure {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("account pending approval"))
		}
	}
	ctx = WithUserID(ctx, userID)
	ctx = WithRole(ctx, u.Role)
	return ctx, nil
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if exemptProcedures[req.Spec().Procedure] {
			return next(ctx, req)
		}
		ctx, err := i.authenticate(ctx, req.Spec().Procedure, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if exemptProcedures[conn.Spec().Procedure] {
			return next(ctx, conn)
		}
		ctx, err := i.authenticate(ctx, conn.Spec().Procedure, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}
