package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/fitter/svc/internal/auth"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/ingest"
	"github.com/google/uuid"
)

const oauthStateCookie = "fitter_oauth_state"

// Login redirects the browser to Strava's OAuth consent screen.
func (d *Deps) Login(w http.ResponseWriter, r *http.Request) {
	state := randomState()
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/auth/strava",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
	callbackURL := d.PublicBaseURL + "/auth/strava/callback"
	http.Redirect(w, r, d.Strava.AuthCodeURL(callbackURL, state), http.StatusFound)
}

// Callback handles Strava's OAuth redirect: exchanges the code, upserts the
// user, kicks off an activity backfill, and hands the browser a JWT.
func (d *Deps) Callback(w http.ResponseWriter, r *http.Request) {
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Redirect(w, r, d.FrontendURL+"/login?error="+errParam, http.StatusFound)
		return
	}

	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/auth/strava", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	tok, err := d.Strava.ExchangeCode(r.Context(), code)
	if err != nil {
		slog.Error("strava code exchange failed", "err", err)
		http.Error(w, "oauth exchange failed", http.StatusBadGateway)
		return
	}
	if tok.Athlete == nil {
		http.Error(w, "strava response missing athlete", http.StatusBadGateway)
		return
	}

	u, err := d.Queries.UpsertUserByStravaID(r.Context(), db.UpsertUserByStravaIDParams{
		ID:              uuid.NewString(),
		StravaAthleteID: tok.Athlete.ID,
		Name:            tok.Athlete.Name(),
		AvatarUrl:       tok.Athlete.ProfileURL,
		AccessToken:     tok.AccessToken,
		RefreshToken:    tok.RefreshToken,
		TokenExpiresAt:  tok.ExpiresAt,
		CreatedAt:       time.Now().Unix(),
	})
	if err != nil {
		slog.Error("upsert user failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	go d.backfillActivities(u)

	jwtToken, err := auth.Issue(d.JWTSecret, u.ID, d.JWTTTL)
	if err != nil {
		slog.Error("issue jwt failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, d.FrontendURL+"/auth/callback#token="+jwtToken, http.StatusFound)
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

func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
