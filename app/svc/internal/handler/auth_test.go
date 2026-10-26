package handler_test

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/fitter/svc/internal/handler"
	"github.com/fitter/svc/internal/strava"
)

// newFakeStrava stands in for Strava's OAuth host. The token endpoint always fails,
// so any callback that gets past the state and scope checks ends at error=exchange.
func newFakeStrava(t *testing.T) *strava.Client {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(fake.Close)
	return strava.NewClient(strava.Config{AuthBaseURL: fake.URL})
}

// newDeps mounts the routes the way main.go does: under /api, with the prefix stripped.
func newDeps(t *testing.T, publicBaseURL string) *handler.Deps {
	t.Helper()
	return &handler.Deps{
		Strava:        newFakeStrava(t),
		JWTSecret:     []byte("test-secret"),
		FrontendURL:   "https://app.example",
		PublicBaseURL: publicBaseURL,
	}
}

func mountAPI(d *handler.Deps) http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("/auth/strava/login", d.Login)
	api.HandleFunc("/auth/strava/callback", d.Callback)
	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", api))
	return root
}

func noRedirect(client *http.Client) *http.Client {
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return client
}

func location(t *testing.T, resp *http.Response) *url.URL {
	t.Helper()
	loc, err := resp.Location()
	if err != nil {
		t.Fatalf("no Location header: %v", err)
	}
	return loc
}

func TestSignIn_StateCookieSurvivesCDNPrefix(t *testing.T) {
	var h http.Handler
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)
	h = mountAPI(newDeps(t, srv.URL+"/api"))

	jar, _ := cookiejar.New(nil)
	client := noRedirect(&http.Client{Jar: jar})

	login, err := client.Get(srv.URL + "/api/auth/strava/login")
	if err != nil {
		t.Fatal(err)
	}
	login.Body.Close()
	state := location(t, login).Query().Get("state")

	resp, err := client.Get(srv.URL + "/api/auth/strava/callback?code=abc&state=" + state)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := location(t, resp).Query().Get("error"); got != "exchange" {
		t.Fatalf("error = %q, want exchange (state cookie not sent back under /api)", got)
	}
}

func TestCallback_ErrorCodes(t *testing.T) {
	d := newDeps(t, "https://app.example")

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"strava denial passes through", "error=access_denied", "access_denied"},
		{"missing state cookie", "code=abc&state=nope", "state"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/auth/strava/callback?"+tc.query, nil)
			rec := httptest.NewRecorder()
			d.Callback(rec, req)
			if got := location(t, rec.Result()).Query().Get("error"); got != tc.want {
				t.Fatalf("error = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCallback_ScopeWithoutActivityAccess(t *testing.T) {
	d := newDeps(t, "https://app.example")

	login := httptest.NewRecorder()
	d.Login(login, httptest.NewRequest(http.MethodGet, "/auth/strava/login", nil))
	state := location(t, login.Result()).Query().Get("state")
	stateCookie := login.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/auth/strava/callback?code=abc&scope=read&state="+state, nil)
	req.AddCookie(stateCookie)
	rec := httptest.NewRecorder()
	d.Callback(rec, req)
	if got := location(t, rec.Result()).Query().Get("error"); got != "scope" {
		t.Fatalf("error = %q, want scope", got)
	}
}

func TestLogin_CookieFollowsPublicBaseURL(t *testing.T) {
	cases := []struct {
		name       string
		base       string
		wantPath   string
		wantSecure bool
	}{
		{"https behind CDN", "https://app.example/api", "/api/auth/strava", true},
		{"plain http at root", "http://localhost:8080", "/auth/strava", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDeps(t, tc.base)
			rec := httptest.NewRecorder()
			d.Login(rec, httptest.NewRequest(http.MethodGet, "/auth/strava/login", nil))
			cookies := rec.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("cookies = %d, want 1", len(cookies))
			}
			c := cookies[0]
			if c.Path != tc.wantPath || c.Secure != tc.wantSecure {
				t.Fatalf("cookie path=%q secure=%v, want path=%q secure=%v",
					c.Path, c.Secure, tc.wantPath, tc.wantSecure)
			}
		})
	}
}
