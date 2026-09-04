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
	"github.com/fitter/svc/internal/strava"
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

// backfillActivities pulls recent history for a newly (re-)authorized user.
// Runs detached from the request so the OAuth redirect isn't held up by
// however many pages Strava takes to page through.
func (d *Deps) backfillActivities(u db.User) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	const perPage = 50
	const maxPages = 4 // ~200 most recent activities
	for page := 1; page <= maxPages; page++ {
		activities, err := d.Strava.ListActivities(ctx, u.AccessToken, page, perPage)
		if err != nil {
			slog.Error("backfill list activities failed", "user_id", u.ID, "page", page, "err", err)
			return
		}
		if len(activities) == 0 {
			return
		}
		for _, a := range activities {
			if err := upsertStravaActivity(ctx, d.Queries, u.ID, a); err != nil {
				slog.Error("backfill upsert activity failed", "user_id", u.ID, "activity_id", a.ID, "err", err)
			}
		}
		if len(activities) < perPage {
			return
		}
	}
}

func upsertStravaActivity(ctx context.Context, q *db.Queries, userID string, a strava.Activity) error {
	startDate, err := time.Parse(time.RFC3339, a.StartDate)
	if err != nil {
		startDate = time.Now()
	}
	_, err = q.UpsertActivity(ctx, db.UpsertActivityParams{
		ID:               uuid.NewString(),
		UserID:           userID,
		StravaActivityID: a.ID,
		Type:             a.Type,
		DistanceM:        a.Distance,
		ElevationM:       a.TotalElevationGain,
		MovingTimeS:      a.MovingTime,
		StartDate:        startDate.Unix(),
		Name:             a.Name,
		CreatedAt:        time.Now().Unix(),
	})
	return err
}

func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
