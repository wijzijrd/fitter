package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/fitter/svc/internal/ingest"
)

// Webhook handles both Strava's one-time subscription validation (GET) and
// its activity push events (POST). See https://developers.strava.com/docs/webhooks/
func (d *Deps) Webhook(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		d.webhookChallenge(w, r)
	case http.MethodPost:
		d.webhookEvent(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (d *Deps) webhookChallenge(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("hub.mode") != "subscribe" || q.Get("hub.verify_token") != d.Strava.VerifyToken() {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"hub.challenge": q.Get("hub.challenge")})
}

func (d *Deps) webhookEvent(w http.ResponseWriter, r *http.Request) {
	var evt struct {
		ObjectType string `json:"object_type"`
		ObjectID   int64  `json:"object_id"`
		AspectType string `json:"aspect_type"`
		OwnerID    int64  `json:"owner_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// Strava requires a fast 2xx ack; do the actual work off the request.
	w.WriteHeader(http.StatusOK)

	if evt.ObjectType != "activity" {
		return
	}
	go d.processActivityEvent(evt.OwnerID, evt.ObjectID, evt.AspectType)
}

func (d *Deps) processActivityEvent(ownerID, activityID int64, aspectType string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if aspectType == "delete" {
		if err := d.Queries.DeleteActivityByStravaID(ctx, activityID); err != nil {
			slog.Error("webhook delete activity failed", "activity_id", activityID, "err", err)
		}
		return
	}

	u, err := d.Queries.GetUserByStravaID(ctx, ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return // event for an athlete who hasn't logged into fitter
	}
	if err != nil {
		slog.Error("webhook lookup user failed", "owner_id", ownerID, "err", err)
		return
	}

	accessToken, err := ingest.EnsureFreshAccessToken(ctx, d.Queries, d.Strava, u)
	if err != nil {
		slog.Error("webhook refresh token failed", "user_id", u.ID, "err", err)
		return
	}
	a, err := d.Strava.GetActivity(ctx, accessToken, activityID)
	if err != nil {
		slog.Error("webhook fetch activity failed", "activity_id", activityID, "err", err)
		return
	}
	if err := ingest.UpsertActivity(ctx, d.Queries, u.ID, *a); err != nil {
		slog.Error("webhook upsert activity failed", "activity_id", activityID, "err", err)
	}
}
