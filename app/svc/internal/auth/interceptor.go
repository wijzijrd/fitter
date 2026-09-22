package auth

import (
	"context"
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
func NewInterceptor(secret []byte, queries *db.Queries) connect.Interceptor {
	return &interceptor{secret: secret, queries: queries}
}

type interceptor struct {
	secret  []byte
	queries *db.Queries
}

func (i *interceptor) authenticate(ctx context.Context, header interface{ Get(string) string }) (context.Context, error) {
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
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, ErrInvalidToken)
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
		ctx, err := i.authenticate(ctx, req.Header())
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
		ctx, err := i.authenticate(ctx, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}
