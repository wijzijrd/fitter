package auth

import (
	"context"
	"strings"

	"connectrpc.com/connect"
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
// (except exemptProcedures), and injects the authenticated user id into the
// request context.
func NewInterceptor(secret []byte) connect.Interceptor {
	return &interceptor{secret: secret}
}

type interceptor struct {
	secret []byte
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
	return WithUserID(ctx, userID), nil
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
