package service

import (
	"context"
	"fmt"
	"time"

	"github.com/fitter/svc/internal/db"
)

// refreshBuffer is how far ahead of actual expiry we proactively refresh, so a
// request never races a token that expires mid-call.
const refreshBuffer = 5 * time.Minute

// ensureFreshAccessToken returns a Strava access token guaranteed valid for at
// least refreshBuffer, refreshing and persisting it first if needed.
func (d *Deps) ensureFreshAccessToken(ctx context.Context, u db.User) (string, error) {
	if time.Now().Add(refreshBuffer).Before(time.Unix(u.TokenExpiresAt, 0)) {
		return u.AccessToken, nil
	}
	tok, err := d.Strava.RefreshAccessToken(ctx, u.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("refresh strava token: %w", err)
	}
	if err := d.Queries.UpdateUserTokens(ctx, db.UpdateUserTokensParams{
		AccessToken:    tok.AccessToken,
		RefreshToken:   tok.RefreshToken,
		TokenExpiresAt: tok.ExpiresAt,
		ID:             u.ID,
	}); err != nil {
		return "", fmt.Errorf("persist refreshed token: %w", err)
	}
	return tok.AccessToken, nil
}
