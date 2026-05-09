package casbin_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	casbinmock "github.com/sergeyslonimsky/elara/internal/service/auth/casbin/mocks"
)

// newBootstrapAdapter creates a MockAdapter that:
//   - returns nil from LoadPolicy (empty storage), triggering built-in seeding
//   - accepts SavePolicy (seed save)
//   - allows any AddPolicy/RemovePolicy/RemoveFilteredPolicy calls from AutoSave
func newBootstrapAdapter(t *testing.T, ctrl *gomock.Controller) *casbinmock.MockAdapter {
	t.Helper()

	adapter := casbinmock.NewMockAdapter(ctrl)
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
		name       string
		email      string
		adminEmail string
		setup      func(e *casbin.Enforcer)
		wantAdmin  bool
	}{
		{
			name:       "email matches adminEmail without existing role assigns admin",
			email:      "admin@example.com",
			adminEmail: "admin@example.com",
			wantAdmin:  true,
		},
		{
			name:       "email matches adminEmail already has role:admin is idempotent",
			email:      "admin@example.com",
			adminEmail: "admin@example.com",
			setup: func(e *casbin.Enforcer) {
				require.NoError(t, e.AddRoleForUser("admin@example.com", "admin", "*"))
			},
			wantAdmin: true,
		},
		{
			name:       "email does not match adminEmail does not assign role",
			email:      "user@example.com",
			adminEmail: "admin@example.com",
			wantAdmin:  false,
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

			err = casbin.CheckBootstrapAdmin(t.Context(), tc.email, tc.adminEmail, e)
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
	adminEmail := email

	require.NoError(t, casbin.CheckBootstrapAdmin(t.Context(), email, adminEmail, e))
	require.NoError(t, casbin.CheckBootstrapAdmin(t.Context(), email, adminEmail, e))

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
