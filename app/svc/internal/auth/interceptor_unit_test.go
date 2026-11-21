package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/fitter/svc/internal/db"
)

type stubUsers struct{ err error }

func (s stubUsers) GetUserByID(context.Context, string) (db.User, error) {
	return db.User{Role: "user"}, s.err
}

func TestAuthenticateLookupErrors(t *testing.T) {
	secret := []byte("test-secret-test-secret-test-secret")
	tok, err := Issue(secret, "u1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+tok)

	cases := []struct {
		name string
		err  error
		want connect.Code
	}{
		{"missing user", sql.ErrNoRows, connect.CodeUnauthenticated},
		{"db blip", errors.New("conn reset"), connect.CodeUnavailable},
	}
	for _, c := range cases {
		i := &interceptor{secret: secret, queries: stubUsers{err: c.err}}
		_, err := i.authenticate(context.Background(), "/fitter.v1.UserService/GetCurrentUser", h)
		if got := connect.CodeOf(err); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
