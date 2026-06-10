package etcdv3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestKVServer_CheckAccess_ServiceToken(t *testing.T) {
	t.Parallel()

	s := &KVServer{}

	tests := []struct {
		name      string
		claims    *auth2.Claims
		namespace string
		action    domain.Action
		wantCode  codes.Code
	}{
		{
			name:      "allow all when no claims (auth disabled)",
			claims:    nil,
			namespace: "prod",
			action:    "read",
			wantCode:  codes.OK,
		},
		{
			name: "allow matching namespace",
			claims: &auth2.Claims{
				Namespaces: []string{"prod"},
				Role:       "reader",
			},
			namespace: "prod",
			action:    "read",
			wantCode:  codes.OK,
		},
		{
			name: "allow star wildcard",
			claims: &auth2.Claims{
				Namespaces: []string{"*"},
				Role:       "writer",
			},
			namespace: "secret",
			action:    "write",
			wantCode:  codes.OK,
		},
		{
			name: "deny non-matching namespace",
			claims: &auth2.Claims{
				Namespaces: []string{"prod"},
				Role:       "reader",
			},
			namespace: "staging",
			action:    "read",
			wantCode:  codes.PermissionDenied,
		},
		{
			name: "deny write for reader role",
			claims: &auth2.Claims{
				Namespaces: []string{"prod"},
				Role:       "reader",
			},
			namespace: "prod",
			action:    "write",
			wantCode:  codes.PermissionDenied,
		},
		{
			name: "deny user claims (not service token)",
			claims: &auth2.Claims{
				Email: "user@example.com",
			},
			namespace: "prod",
			action:    "read",
			wantCode:  codes.PermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if tt.claims != nil {
				ctx = auth2.WithClaims(ctx, tt.claims)
			}

			err := s.checkAccess(ctx, tt.namespace, tt.action)
			if tt.wantCode == codes.OK {
				assert.NoError(t, err)
			} else {
				assert.Equal(t, tt.wantCode, status.Code(err))
			}
		})
	}
}
