// Package strava is a minimal client for the parts of the Strava v3 API
// fitter needs: OAuth token exchange/refresh, activity list/detail fetch,
// and webhook event handling.
package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	authBaseURL = "https://www.strava.com/oauth"
	apiBaseURL  = "https://www.strava.com/api/v3"
)

type Config struct {
	ClientID           string
	ClientSecret       string
	WebhookVerifyToken string
}

type Client struct {
	cfg  Config
	http *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 15 * time.Second}}
}

// TokenResponse mirrors Strava's OAuth token exchange/refresh response.
type TokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresAt    int64    `json:"expires_at"`
	Athlete      *Athlete `json:"athlete,omitempty"`
}

type Athlete struct {
	ID         int64  `json:"id"`
	FirstName  string `json:"firstname"`
	LastName   string `json:"lastname"`
	ProfileURL string `json:"profile"`
}

func (a Athlete) Name() string {
	name := a.FirstName
	if a.LastName != "" {
		if name != "" {
			name += " "
		}
		name += a.LastName
	}
	return name
}

// AuthCodeURL returns the URL to redirect the user to for Strava's OAuth consent screen.
func (c *Client) AuthCodeURL(redirectURI, state string) string {
	v := url.Values{
		"client_id":       {c.cfg.ClientID},
		"redirect_uri":    {redirectURI},
		"response_type":   {"code"},
		"approval_prompt": {"auto"},
		"scope":           {"read,activity:read_all"},
		"state":           {state},
	}
	return authBaseURL + "/authorize?" + v.Encode()
}

// ExchangeCode trades an OAuth authorization code for tokens + the athlete profile.
func (c *Client) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	v := url.Values{
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	}
	return c.postToken(ctx, v)
}

// RefreshAccessToken trades a refresh token for a new access token.
func (c *Client) RefreshAccessToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	v := url.Values{
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}
	return c.postToken(ctx, v)
}

func (c *Client) postToken(ctx context.Context, v url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authBaseURL+"/token", nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = v.Encode()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("strava token request failed: status %d", resp.StatusCode)
	}
	var tok TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &tok, nil
}

// Activity is the subset of Strava's activity summary/detail fields fitter tracks.
type Activity struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Type               string  `json:"type"`
	Distance           float64 `json:"distance"`             // meters
	TotalElevationGain float64 `json:"total_elevation_gain"` // meters
	MovingTime         int64   `json:"moving_time"`          // seconds
	StartDate          string  `json:"start_date"`           // RFC3339
}

// GetActivity fetches full detail for a single activity, using the owner's access token.
func (c *Client) GetActivity(ctx context.Context, accessToken string, activityID int64) (*Activity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/activities/%d", apiBaseURL, activityID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get activity %d failed: status %d", activityID, resp.StatusCode)
	}
	var a Activity
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return nil, fmt.Errorf("decode activity: %w", err)
	}
	return &a, nil
}

// ListActivities fetches one page of the athlete's activities, most recent first.
func (c *Client) ListActivities(ctx context.Context, accessToken string, page, perPage int) ([]Activity, error) {
	v := url.Values{
		"page":     {strconv.Itoa(page)},
		"per_page": {strconv.Itoa(perPage)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		apiBaseURL+"/athlete/activities?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list activities failed: status %d", resp.StatusCode)
	}
	var activities []Activity
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		return nil, fmt.Errorf("decode activities: %w", err)
	}
	return activities, nil
}
