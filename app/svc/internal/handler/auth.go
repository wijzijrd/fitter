package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/ingest"
	"github.com/google/uuid"
)

const oauthStateCookie = "fitter_oauth_state"

// Codes Callback sends back to the frontend. LoginView.vue `messages` maps these.
const (
	errState    = "state"
	errScope    = "scope"
	errExchange = "exchange"
	errServer   = "server"
)

// Login redirects the browser to Strava's OAuth consent screen.
func (d *Deps) Login(w http.ResponseWriter, r *http.Request) {
	state := randomState()
	http.SetCookie(w, d.stateCookie(state, 600))
	callbackURL := d.PublicBaseURL + "/auth/strava/callback"
	http.Redirect(w, r, d.Strava.AuthCodeURL(callbackURL, state), http.StatusFound)
}

// Callback handles Strava's OAuth redirect: exchanges the code, upserts the
// user, kicks off an activity backfill, and hands the browser a JWT.
func (d *Deps) Callback(w http.ResponseWriter, r *http.Request) {
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		d.loginError(w, r, errParam)
		return
	}

	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		slog.Warn("oauth state mismatch", "err", err, "cookie_present", err == nil)
		d.loginError(w, r, errState)
		return
	}
	http.SetCookie(w, d.stateCookie("", -1))

	code := r.URL.Query().Get("code")
	if code == "" {
		d.loginError(w, r, errExchange)
		return
	}

	// Strava echoes the scopes the user kept ticked. Without activity access the feed stays empty.
	if s := r.URL.Query().Get("scope"); s != "" && !strings.Contains(s, "activity:read") {
		d.loginError(w, r, errScope)
		return
	}

	tok, err := d.Strava.ExchangeCode(r.Context(), code)
	if err != nil {
		slog.Error("strava code exchange failed", "err", err)
		d.loginError(w, r, errExchange)
		return
	}
	if tok.Athlete == nil {
		slog.Error("strava token response has no athlete")
		d.loginError(w, r, errExchange)
		return
	}

	var u db.User
	if d.isBootstrapAdmin(tok.Athlete.ID) {
		// Explicitly (re-)grants admin on every login for a listed athlete
		// id. Never the other branch: removing an id from the env var only
		// stops future re-grants via this path, it never demotes a
		// standing admin (see UpsertUserByStravaID's own doc comment).
		u, err = d.Queries.UpsertUserByStravaIDAsAdmin(r.Context(), db.UpsertUserByStravaIDAsAdminParams{
			ID:              uuid.NewString(),
			StravaAthleteID: tok.Athlete.ID,
			Name:            tok.Athlete.Name(),
			AvatarUrl:       tok.Athlete.ProfileURL,
			AccessToken:     tok.AccessToken,
			RefreshToken:    tok.RefreshToken,
			TokenExpiresAt:  tok.ExpiresAt,
			CreatedAt:       time.Now().Unix(),
		})
	} else {
		// Never mentions role at all, so an existing user's role (in
		// particular a promoted admin's) is left untouched here.
		u, err = d.Queries.UpsertUserByStravaID(r.Context(), db.UpsertUserByStravaIDParams{
			ID:              uuid.NewString(),
			StravaAthleteID: tok.Athlete.ID,
			Name:            tok.Athlete.Name(),
			AvatarUrl:       tok.Athlete.ProfileURL,
			AccessToken:     tok.AccessToken,
			RefreshToken:    tok.RefreshToken,
			TokenExpiresAt:  tok.ExpiresAt,
			CreatedAt:       time.Now().Unix(),
		})
	}
	if err != nil {
		slog.Error("upsert user failed", "err", err)
		d.loginError(w, r, errServer)
		return
	}

	go d.backfillActivities(u)

	jwtToken, err := auth.Issue(d.JWTSecret, u.ID, d.JWTTTL)
	if err != nil {
		slog.Error("issue jwt failed", "err", err)
		d.loginError(w, r, errServer)
		return
	}
	http.Redirect(w, r, d.FrontendURL+"/auth/callback#token="+jwtToken, http.StatusFound)
}

// loginError sends the browser back to the frontend login page with a short
// code it maps to copy, instead of a plain-text API response nobody can act on.
func (d *Deps) loginError(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, d.FrontendURL+"/login?error="+url.QueryEscape(code), http.StatusFound)
}

// stateCookie builds the OAuth state cookie for the callback path under PublicBaseURL.
// Behind the CDN the API lives under /api, so the browser only sends the cookie to
// /api/auth/strava/... when the path follows the public base.
func (d *Deps) stateCookie(value string, maxAge int) *http.Cookie {
	base, err := url.Parse(d.PublicBaseURL)
	if err != nil {
		base = &url.URL{}
	}
	return &http.Cookie{
		Name:     oauthStateCookie,
		Value:    value,
		Path:     strings.TrimSuffix(base.Path, "/") + "/auth/strava",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   base.Scheme == "https",
		SameSite: http.SameSiteLaxMode,
	}
}

// backfillActivities pulls recent history for a newly (re-)authorized user
// via internal/ingest.Backfill — the single Strava write path. Runs detached
// from the request so the OAuth redirect isn't held up by however many pages
// Strava takes to page through.
func (d *Deps) backfillActivities(u db.User) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	count, err := ingest.Backfill(ctx, d.Queries, d.Strava, u, 0)
	if err != nil {
		slog.Error("backfill activities failed", "user_id", u.ID, "err", err)
		return
	}
	slog.Info("backfill activities complete", "user_id", u.ID, "count", count)
}

// isBootstrapAdmin reports whether athleteID is in ADMIN_STRAVA_ATHLETE_IDS.
func (d *Deps) isBootstrapAdmin(athleteID int64) bool {
	for _, id := range d.AdminStravaAthleteIDs {
		if id == athleteID {
			return true
		}
	}
	return false
}

func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
