package casbin_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
	casbin_mock "github.com/sergeyslonimsky/elara/internal/auth/casbin/mocks"
)

// newBootstrapAdapter creates a MockAdapter that:
//   - returns nil from LoadPolicy (empty storage), triggering built-in seeding
//   - accepts SavePolicy (seed save)
//   - allows any AddPolicy/RemovePolicy/RemoveFilteredPolicy calls from AutoSave
func newBootstrapAdapter(t *testing.T, ctrl *gomock.Controller) *casbin_mock.MockAdapter {
	t.Helper()

	adapter := casbin_mock.NewMockAdapter(ctrl)
	adapter.EXPECT().LoadPolicy(gomock.Any()).Return(nil)
	adapter.EXPECT().SavePolicy(gomock.Any()).Return(nil)
	adapter.EXPECT().AddPolicy(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	adapter.EXPECT().RemovePolicy(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	adapter.EXPECT().RemoveFilteredPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return adapter
}

func TestCheckBootstrapAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		email       string
		adminEmails []string
		setup       func(e *casbin.Enforcer)
		wantAdmin   bool
	}{
		{
			name:        "email in adminEmails without existing role assigns admin",
			email:       "admin@example.com",
			adminEmails: []string{"admin@example.com"},
			wantAdmin:   true,
		},
		{
			name:        "email in adminEmails already has role:admin is idempotent",
			email:       "admin@example.com",
			adminEmails: []string{"admin@example.com"},
			setup: func(e *casbin.Enforcer) {
				require.NoError(t, e.AddRoleForUser("admin@example.com", "admin", "*"))
			},
			wantAdmin: true,
		},
		{
			name:        "email not in adminEmails does not assign role",
			email:       "user@example.com",
			adminEmails: []string{"admin@example.com"},
			wantAdmin:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			adapter := newBootstrapAdapter(t, ctrl)

			e, err := casbin.NewEnforcer(adapter)
			require.NoError(t, err)

			if tc.setup != nil {
				tc.setup(e)
			}

			err = casbin.CheckBootstrapAdmin(t.Context(), tc.email, tc.adminEmails, e)
			require.NoError(t, err)

			roles, err := e.GetRolesForUser(tc.email, "*")
			require.NoError(t, err)

			hasAdmin := slices.Contains(roles, "admin")

			assert.Equal(t, tc.wantAdmin, hasAdmin)
		})
	}
}

func TestCheckBootstrapAdmin_NoDuplicateAssignment(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	adapter := newBootstrapAdapter(t, ctrl)

	e, err := casbin.NewEnforcer(adapter)
	require.NoError(t, err)

	email := "admin@example.com"
	adminEmails := []string{email}

	require.NoError(t, casbin.CheckBootstrapAdmin(t.Context(), email, adminEmails, e))
	require.NoError(t, casbin.CheckBootstrapAdmin(t.Context(), email, adminEmails, e))

	roles, err := e.GetRolesForUser(email, "*")
	require.NoError(t, err)

	count := 0

	for _, r := range roles {
		if r == "admin" {
			count++
		}
	}

	assert.Equal(t, 1, count, "role:admin should only be assigned once")
}
