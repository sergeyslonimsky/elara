package interceptor_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/interceptor"
	interceptormock "github.com/sergeyslonimsky/elara/internal/handler/v2/interceptor/mocks" //nolint:revive // generated package name is mock_interceptor
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

const testProc = "/elara.access.v1.AccessService/AssignRole"

func TestRBACInterceptor_Authorize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		registry    map[string]interceptor.Permission
		authOnly    map[string]struct{}
		ctxFunc     func() context.Context
		mockEnforce func(enforcer *interceptormock.MockrbacEnforcer)
		wantErrCode connect.Code
	}{
		{
			name: "registered procedure + Enforce true -> nil error",
			registry: map[string]interceptor.Permission{
				testProc: {Object: domain.ObjectPolicy, Action: domain.ActionWrite},
			},
			ctxFunc: func() context.Context {
				return auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})
			},
			mockEnforce: func(enforcer *interceptormock.MockrbacEnforcer) {
				enforcer.EXPECT().
					Enforce("admin@example.com", domain.DomainAll, domain.ObjectPolicy, domain.ActionWrite).
					Return(true, nil)
			},
		},
		{
			name: "registered procedure + Enforce false -> PermissionDenied",
			registry: map[string]interceptor.Permission{
				testProc: {Object: domain.ObjectPolicy, Action: domain.ActionWrite},
			},
			ctxFunc: func() context.Context {
				return auth.WithClaims(context.Background(), &auth.Claims{Email: "user@example.com"})
			},
			mockEnforce: func(enforcer *interceptormock.MockrbacEnforcer) {
				enforcer.EXPECT().
					Enforce("user@example.com", domain.DomainAll, domain.ObjectPolicy, domain.ActionWrite).
					Return(false, nil)
			},
			wantErrCode: connect.CodePermissionDenied,
		},
		{
			name: "registered procedure, no claims -> Unauthenticated",
			registry: map[string]interceptor.Permission{
				testProc: {Object: domain.ObjectPolicy, Action: domain.ActionWrite},
			},
			ctxFunc:     context.Background,
			wantErrCode: connect.CodeUnauthenticated,
		},
		{
			name: "registered procedure + Enforce error -> Internal",
			registry: map[string]interceptor.Permission{
				testProc: {Object: domain.ObjectPolicy, Action: domain.ActionWrite},
			},
			ctxFunc: func() context.Context {
				return auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})
			},
			mockEnforce: func(enforcer *interceptormock.MockrbacEnforcer) {
				enforcer.EXPECT().
					Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(false, errors.New("casbin down"))
			},
			wantErrCode: connect.CodeInternal,
		},
		{
			name:     "auth-only procedure -> nil error without Enforce",
			authOnly: map[string]struct{}{testProc: {}},
			ctxFunc: func() context.Context {
				return auth.WithClaims(context.Background(), &auth.Claims{Email: "anyone@example.com"})
			},
		},
		{
			name:     "unclassified procedure -> PermissionDenied (fail-closed)",
			registry: map[string]interceptor.Permission{},
			authOnly: map[string]struct{}{},
			ctxFunc: func() context.Context {
				return auth.WithClaims(context.Background(), &auth.Claims{Email: "admin@example.com"})
			},
			wantErrCode: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := interceptormock.NewMockrbacEnforcer(ctrl)

			if tt.mockEnforce != nil {
				tt.mockEnforce(enforcer)
			}

			rbac := interceptor.NewRBACInterceptor(enforcer, tt.registry, tt.authOnly)
			err := rbac.Authorize(tt.ctxFunc(), testProc)

			if tt.wantErrCode == 0 {
				require.NoError(t, err)

				return
			}
			var connectErr *connect.Error
			require.ErrorAs(t, err, &connectErr)
			assert.Equal(t, tt.wantErrCode, connectErr.Code())
		})
	}
}

func TestRBACInterceptor_OverlapPanics(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, `rbac: procedure "/x" listed both in registry and auth-only whitelist`, func() {
		interceptor.NewRBACInterceptor(
			nil,
			map[string]interceptor.Permission{"/x": {Object: "y", Action: "z"}},
			map[string]struct{}{"/x": {}},
		)
	})
}

func TestDefaultRBACPolicies_NoOverlapWithAuthOnly(t *testing.T) {
	t.Parallel()

	enforced := interceptor.DefaultRBACPolicies()
	authOnly := interceptor.DefaultRBACAuthOnly()

	for proc := range enforced {
		_, dup := authOnly[proc]
		assert.Falsef(t, dup, "procedure %q is in both registry and auth-only whitelist", proc)
	}

	assert.NotEmpty(t, enforced)
	assert.NotEmpty(t, authOnly)
}
