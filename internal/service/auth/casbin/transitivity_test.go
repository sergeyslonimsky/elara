package casbin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
)

func TestEnforcer_GroupMembershipTransitivity(t *testing.T) {
	t.Parallel()

	type enforceFields struct {
		subject string
		domain  string
		object  string
		action  string
	}

	tests := []struct {
		name          string
		setupFunc     func(t *testing.T) *casbin.Enforcer
		enforceFields enforceFields
		hasAccess     bool
	}{
		{
			name: "member inherits group role",
			setupFunc: func(t *testing.T) *casbin.Enforcer {
				t.Helper()
				e := newTestEnforcer(t, nil)
				require.NoError(t, e.AddRoleForUser("devops", "writer", "prod"))
				require.NoError(t, e.AddRoleForUser("alice@example.com", "devops", "prod"))

				return e
			},
			enforceFields: enforceFields{
				subject: "alice@example.com",
				domain:  "prod",
				object:  "config",
				action:  "write",
			},
			hasAccess: true,
		},
		{
			name: "removing membership revokes access",
			setupFunc: func(t *testing.T) *casbin.Enforcer {
				t.Helper()
				e := newTestEnforcer(t, nil)
				require.NoError(t, e.AddRoleForUser("devops", "writer", "prod"))
				require.NoError(t, e.AddRoleForUser("alice@example.com", "devops", "prod"))
				require.NoError(t, e.RemoveRoleForUser("alice@example.com", "devops", "prod"))

				return e
			},
			enforceFields: enforceFields{
				subject: "alice@example.com",
				domain:  "prod",
				object:  "config",
				action:  "write",
			},
			hasAccess: false,
		},
		{
			name: "member in two groups keeps access when removed from one",
			setupFunc: func(t *testing.T) *casbin.Enforcer {
				t.Helper()
				e := newTestEnforcer(t, nil)
				require.NoError(t, e.AddRoleForUser("devops", "writer", "prod"))
				require.NoError(t, e.AddRoleForUser("ops", "writer", "prod"))
				require.NoError(t, e.AddRoleForUser("alice@example.com", "devops", "prod"))
				require.NoError(t, e.AddRoleForUser("alice@example.com", "ops", "prod"))
				require.NoError(t, e.RemoveRoleForUser("alice@example.com", "devops", "prod"))

				return e
			},
			enforceFields: enforceFields{
				subject: "alice@example.com",
				domain:  "prod",
				object:  "config",
				action:  "write",
			},
			hasAccess: true,
		},
		{
			name: "no access after removing from all groups",
			setupFunc: func(t *testing.T) *casbin.Enforcer {
				t.Helper()
				e := newTestEnforcer(t, nil)
				require.NoError(t, e.AddRoleForUser("devops", "writer", "prod"))
				require.NoError(t, e.AddRoleForUser("ops", "writer", "prod"))
				require.NoError(t, e.AddRoleForUser("alice@example.com", "devops", "prod"))
				require.NoError(t, e.AddRoleForUser("alice@example.com", "ops", "prod"))
				require.NoError(t, e.RemoveRoleForUser("alice@example.com", "devops", "prod"))
				require.NoError(t, e.RemoveRoleForUser("alice@example.com", "ops", "prod"))

				return e
			},
			enforceFields: enforceFields{
				subject: "alice@example.com",
				domain:  "prod",
				object:  "config",
				action:  "write",
			},
			hasAccess: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			e := tc.setupFunc(t)

			hasAccess, err := e.Enforce(
				tc.enforceFields.subject,
				tc.enforceFields.domain,
				tc.enforceFields.object,
				tc.enforceFields.action,
			)
			require.NoError(t, err)
			assert.Equal(t, tc.hasAccess, hasAccess)
		})
	}
}
