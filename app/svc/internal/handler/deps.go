// Package handler holds the plain HTTP endpoints that can't be ConnectRPC
// calls: the Strava OAuth redirect dance (needs real browser navigation) and
// the Strava webhook callback (Strava POSTs directly, not via our clients).
package handler

import (
	"time"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/strava"
)

type Deps struct {
	Queries   *db.Queries
	Strava    *strava.Client
	JWTSecret []byte
	JWTTTL    time.Duration

	// FrontendURL is where we redirect the browser after login, e.g. https://app.fitter.example.
	FrontendURL string
	// PublicBaseURL is this backend's own public URL, used to build the OAuth callback redirect_uri.
	PublicBaseURL string
	// AdminStravaAthleteIDs is the bootstrap admin allowlist (ADMIN_STRAVA_ATHLETE_IDS):
	// a Strava athlete id in this list is (re-)granted role='admin' on every login. See
	// Callback for why removal from this list never demotes a standing admin.
	AdminStravaAthleteIDs []int64
}
