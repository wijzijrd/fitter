package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Event is a webhook push event from Strava. See:
// https://developers.strava.com/docs/webhooks/
type Event struct {
	ObjectType string `json:"object_type"` // "activity" or "athlete"
	ObjectID   int64  `json:"object_id"`
	AspectType string `json:"aspect_type"` // "create", "update", "delete"
	OwnerID    int64  `json:"owner_id"`    // Strava athlete id
	EventTime  int64  `json:"event_time"`
}

// Subscription is Strava's push_subscriptions resource.
type Subscription struct {
	ID          int64  `json:"id"`
	CallbackURL string `json:"callback_url"`
}

// CreateSubscription registers this app's single webhook subscription with Strava.
// Strava allows exactly one subscription per API application, so this is a one-time
// setup step (see cmd/webhooksetup), not something done per user.
func (c *Client) CreateSubscription(ctx context.Context, callbackURL string) (*Subscription, error) {
	v := url.Values{
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
		"callback_url":  {callbackURL},
		"verify_token":  {c.cfg.WebhookVerifyToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.APIBaseURL+"/push_subscriptions", nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = v.Encode()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body := make([]byte, 2048)
		n, _ := resp.Body.Read(body)
		return nil, fmt.Errorf("create subscription failed: status %d: %s", resp.StatusCode, body[:n])
	}
	var sub Subscription
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return nil, fmt.Errorf("decode subscription: %w", err)
	}
	return &sub, nil
}

// ListSubscriptions returns this app's current webhook subscription(s) — normally at most one.
func (c *Client) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	v := url.Values{
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.APIBaseURL+"/push_subscriptions?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list subscriptions failed: status %d", resp.StatusCode)
	}
	var subs []Subscription
	if err := json.NewDecoder(resp.Body).Decode(&subs); err != nil {
		return nil, fmt.Errorf("decode subscriptions: %w", err)
	}
	return subs, nil
}

// DeleteSubscription removes an existing webhook subscription by id.
func (c *Client) DeleteSubscription(ctx context.Context, id int64) error {
	v := url.Values{
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/push_subscriptions/%d?%s", c.cfg.APIBaseURL, id, v.Encode()), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete subscription failed: status %d", resp.StatusCode)
	}
	return nil
}

// VerifyToken returns the token this app expects back in the webhook validation challenge.
func (c *Client) VerifyToken() string {
	return c.cfg.WebhookVerifyToken
}
