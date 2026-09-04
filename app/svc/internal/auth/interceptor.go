package auth

import (
	"context"
	"strings"

	"connectrpc.com/connect"
)

// NewInterceptor returns a Connect interceptor that requires a valid
// "Authorization: Bearer <jwt>" header on every unary and streaming call,
// and injects the authenticated user id into the request context.
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
		ctx, err := i.authenticate(ctx, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}
