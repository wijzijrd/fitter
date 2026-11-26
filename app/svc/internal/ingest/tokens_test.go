package ingest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/dbtest"
	"github.com/fitter/svc/internal/ingest"
	"github.com/fitter/svc/internal/strava"
)

// A copied beta user has blank tokens until they sign in. Refreshing must report that
// instead of calling Strava with an empty refresh token.
func TestEnsureFreshAccessToken_BlankTokensAreNotConnected(t *testing.T) {
	_, q := dbtest.New(t)
	c := strava.NewClient(strava.Config{}) // never called: the blank check comes first
	_, err := ingest.EnsureFreshAccessToken(context.Background(), q, c, db.User{ID: "copied", RefreshToken: ""})
	if !errors.Is(err, ingest.ErrNotConnected) {
		t.Fatalf("err = %v, want ErrNotConnected", err)
	}
}
