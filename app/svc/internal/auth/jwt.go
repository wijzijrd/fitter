// Package auth issues and verifies the app's own bearer JWTs, and carries
// the authenticated user id through request context. This is separate from
// Strava's OAuth tokens (see internal/strava) — the JWT is what the frontend
// sends back to us on every call once a user has logged in via Strava.
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid or expired token")

type claims struct {
	jwt.RegisteredClaims
}

// Issue creates a signed JWT for the given user id, valid for ttl.
func Issue(secret []byte, userID string, ttl time.Duration) (string, error) {
	now := time.Now()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString(secret)
}

// Verify parses and validates a JWT, returning the user id (subject) it was issued for.
func Verify(secret []byte, tokenStr string) (string, error) {
	var c claims
	token, err := jwt.ParseWithClaims(tokenStr, &c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return "", ErrInvalidToken
	}
	if c.Subject == "" {
		return "", ErrInvalidToken
	}
	return c.Subject, nil
}

type contextKey struct{}

var userIDKey = contextKey{}

// WithUserID returns a new context carrying the authenticated user id.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDFromContext returns the authenticated user id set by the auth interceptor.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok && v != ""
}
