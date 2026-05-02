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

// newBootstrapLoader creates a PolicyLoader mock that:
//   - returns the given initial rules on Load()
//   - accepts any number of Save() calls (seeding + CheckBootstrapAdmin persistence)
func newBootstrapLoader(t *testing.T, ctrl *gomock.Controller, initRules [][]string) *casbin_mock.MockPolicyLoader {
	t.Helper()

	loader := casbin_mock.NewMockPolicyLoader(ctrl)
	loader.EXPECT().Load().Return(initRules, nil).AnyTimes()
	loader.EXPECT().Save(gomock.Any()).Return(nil).AnyTimes()

	return loader
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
				require.NoError(t, e.AddRoleForUser("admin@example.com", "role:admin", "*"))
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
			loader := newBootstrapLoader(t, ctrl, nil)

			e, err := casbin.NewEnforcer(loader)
			require.NoError(t, err)

			if tc.setup != nil {
				tc.setup(e)
			}

			err = casbin.CheckBootstrapAdmin(tc.email, tc.adminEmails, e, loader)
			require.NoError(t, err)

			roles, err := e.GetRolesForUser(tc.email, "*")
			require.NoError(t, err)

			hasAdmin := slices.Contains(roles, "role:admin")

			assert.Equal(t, tc.wantAdmin, hasAdmin)
		})
	}
}

func TestCheckBootstrapAdmin_NoDuplicateAssignment(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	loader := newBootstrapLoader(t, ctrl, nil)

	e, err := casbin.NewEnforcer(loader)
	require.NoError(t, err)

	email := "admin@example.com"
	adminEmails := []string{email}

	require.NoError(t, casbin.CheckBootstrapAdmin(email, adminEmails, e, loader))
	require.NoError(t, casbin.CheckBootstrapAdmin(email, adminEmails, e, loader))

	roles, err := e.GetRolesForUser(email, "*")
	require.NoError(t, err)

	count := 0

	for _, r := range roles {
		if r == "role:admin" {
			count++
		}
	}

	assert.Equal(t, 1, count, "role:admin should only be assigned once")
}
