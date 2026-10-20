package service

// Internal-package test (not service_test) because requireAdmin is
// unexported: no admin-gated RPC exists yet (that's a later task), so this
// is the only way to exercise it directly, per Task 3's brief.

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/fitter/svc/internal/auth"
)

func TestRequireAdmin(t *testing.T) {
	cases := []struct {
		name    string
		ctx     context.Context
		wantErr bool
	}{
		{
			name:    "admin role allowed",
			ctx:     auth.WithRole(context.Background(), "admin"),
			wantErr: false,
		},
		{
			name:    "member role denied",
			ctx:     auth.WithRole(context.Background(), "member"),
			wantErr: true,
		},
		{
			name:    "no role in context denied",
			ctx:     context.Background(),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requireAdmin(tc.ctx)
			if tc.wantErr && err == nil {
				t.Fatalf("requireAdmin() = nil, want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("requireAdmin() = %v, want nil", err)
			}
			if err != nil && connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Fatalf("requireAdmin() code = %v, want PermissionDenied", connect.CodeOf(err))
			}
		})
	}
}
