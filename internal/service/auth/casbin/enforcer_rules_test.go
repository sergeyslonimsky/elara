package casbin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

func TestNewEnforcer_WithExistingRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rules [][]string
		// verify describes what to assert after loading
		verify func(t *testing.T, e *casbin.Enforcer, txm storage.TxManager)
	}{
		{
			name: "pre-existing p rule is enforced after load",
			rules: [][]string{
				{"p", "admin", "*", "*", "*"},
				{"g", "alice", "admin", "*"},
			},
			verify: func(t *testing.T, e *casbin.Enforcer, _ storage.TxManager) {
				t.Helper()

				ok, err := e.Enforce("alice", "*", "anything", "delete")
				require.NoError(t, err)
				assert.True(t, ok, "alice should be allowed via loaded p and g rules")
			},
		},
		{
			name: "pre-existing g rule assigns role correctly",
			rules: [][]string{
				{"p", "reader", "*", "config", "read"},
				{"g", "bob", "reader", "prod"},
			},
			verify: func(t *testing.T, e *casbin.Enforcer, _ storage.TxManager) {
				t.Helper()

				roles, err := e.GetRolesForUser("bob", "prod")
				require.NoError(t, err)
				assert.Contains(t, roles, "reader")

				ok, err := e.Enforce("bob", "prod", "config", "read")
				require.NoError(t, err)
				assert.True(t, ok, "bob should be allowed to read config in prod")

				ok, err = e.Enforce("bob", "dev", "config", "read")
				require.NoError(t, err)
				assert.False(t, ok, "bob should not be allowed in dev domain")
			},
		},
		{
			name: "rules with short/malformed entries are skipped without panic",
			rules: [][]string{
				{},
				{"p"},
				{"p", "admin"},
				{"g"},
				{"g", "user"},
				{"p", "writer", "*", "config", "write", "extra"},
				{"p", "writer", "*", "config", "write"},
			},
			verify: func(t *testing.T, e *casbin.Enforcer, txm storage.TxManager) {
				t.Helper()

				seedRole(t, e, txm, "carol", "writer", "*")
				ok, err := e.Enforce("carol", "*", "config", "write")
				require.NoError(t, err)
				assert.True(t, ok, "carol should be allowed via the valid loaded rule")
			},
		},
		{
			name: "multiple p and g rules are all loaded",
			rules: [][]string{
				{"p", "writer", "*", "config", "read"},
				{"p", "writer", "*", "config", "write"},
				{"p", "reader", "*", "config", "read"},
				{"g", "dave", "writer", "*"},
				{"g", "eve", "reader", "*"},
			},
			verify: func(t *testing.T, e *casbin.Enforcer, _ storage.TxManager) {
				t.Helper()

				ok, err := e.Enforce("dave", "*", "config", "write")
				require.NoError(t, err)
				assert.True(t, ok, "dave (editor) should be able to write config")

				ok, err = e.Enforce("eve", "*", "config", "write")
				require.NoError(t, err)
				assert.False(t, ok, "eve (viewer) should not be able to write config")

				ok, err = e.Enforce("eve", "*", "config", "read")
				require.NoError(t, err)
				assert.True(t, ok, "eve (viewer) should be able to read config")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, txm := newTestEnforcerWithTxM(t, tt.rules)

			tt.verify(t, e, txm)
		})
	}
}

func TestEnforcer_PolicyMethods(t *testing.T) {
	t.Parallel()

	t.Run("AddPolicy adds a rule and Enforce recognizes it", func(t *testing.T) {
		t.Parallel()

		e, txm := newTestEnforcerWithTxM(t, nil)

		seedPolicy(t, e, txm, "role:custom", "ns1", "config", "read")

		ok, err := e.Enforce("role:custom", "ns1", "config", "read")
		require.NoError(t, err)
		assert.True(t, ok, "custom policy should be enforced")
	})

	t.Run("RemovePolicy removes a rule and Enforce no longer recognizes it", func(t *testing.T) {
		t.Parallel()

		e, txm := newTestEnforcerWithTxM(t, nil)

		seedPolicy(t, e, txm, "role:temp", "*", "namespace", "write")

		ok, err := e.Enforce("role:temp", "*", "namespace", "write")
		require.NoError(t, err)
		require.True(t, ok, "policy must exist before removal")

		removePolicy(t, e, txm, "role:temp", "*", "namespace", "write")

		ok, err = e.Enforce("role:temp", "*", "namespace", "write")
		require.NoError(t, err)
		assert.False(t, ok, "policy should not be enforced after removal")
	})

	t.Run("multiple AddRoleForUser calls do not duplicate", func(t *testing.T) {
		t.Parallel()

		e, txm := newTestEnforcerWithTxM(t, nil)

		seedRole(t, e, txm, "frank", "writer", "*")
		seedRole(t, e, txm, "frank", "writer", "*")

		roles, err := e.GetRolesForUser("frank", "*")
		require.NoError(t, err)

		count := 0
		for _, r := range roles {
			if r == "writer" {
				count++
			}
		}
		assert.Equal(t, 1, count, "role:editor should appear exactly once for frank")
	})

	t.Run("GetRolesForUser returns correct roles after multiple assignments", func(t *testing.T) {
		t.Parallel()

		e, txm := newTestEnforcerWithTxM(t, nil)

		seedRole(t, e, txm, "grace", "writer", "prod")
		seedRole(t, e, txm, "grace", "reader", "staging")

		prodRoles, err := e.GetRolesForUser("grace", "prod")
		require.NoError(t, err)
		assert.Contains(t, prodRoles, "writer")
		assert.NotContains(t, prodRoles, "reader")

		stagingRoles, err := e.GetRolesForUser("grace", "staging")
		require.NoError(t, err)
		assert.Contains(t, stagingRoles, "reader")
		assert.NotContains(t, stagingRoles, "writer")
	})

	t.Run("GetRolesForUser returns empty slice for unknown user", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)

		roles, err := e.GetRolesForUser("nobody", "*")
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}
