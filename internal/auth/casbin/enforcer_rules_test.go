package casbin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
	casbin_mock "github.com/sergeyslonimsky/elara/internal/auth/casbin/mocks"
)

func TestNewEnforcer_WithExistingRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rules [][]string
		// verify describes what to assert after loading
		verify func(t *testing.T, e *casbin.Enforcer)
	}{
		{
			name: "pre-existing p rule is enforced after load",
			rules: [][]string{
				{"p", "role:admin", "*", "*", "*"},
				{"g", "alice", "role:admin", "*"},
			},
			verify: func(t *testing.T, e *casbin.Enforcer) {
				t.Helper()

				ok, err := e.Enforce("alice", "*", "anything", "delete")
				require.NoError(t, err)
				assert.True(t, ok, "alice should be allowed via loaded p and g rules")
			},
		},
		{
			name: "pre-existing g rule assigns role correctly",
			rules: [][]string{
				{"p", "role:viewer", "*", "config", "read"},
				{"g", "bob", "role:viewer", "prod"},
			},
			verify: func(t *testing.T, e *casbin.Enforcer) {
				t.Helper()

				roles, err := e.GetRolesForUser("bob", "prod")
				require.NoError(t, err)
				assert.Contains(t, roles, "role:viewer")

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
				{"p", "role:admin"},
				{"g"},
				{"g", "user"},
				{"p", "role:editor", "*", "config", "write", "extra"},
				{"p", "role:editor", "*", "config", "write"},
			},
			verify: func(t *testing.T, e *casbin.Enforcer) {
				t.Helper()

				require.NoError(t, e.AddRoleForUser("carol", "role:editor", "*"))
				ok, err := e.Enforce("carol", "*", "config", "write")
				require.NoError(t, err)
				assert.True(t, ok, "carol should be allowed via the valid loaded rule")
			},
		},
		{
			name: "multiple p and g rules are all loaded",
			rules: [][]string{
				{"p", "role:editor", "*", "config", "read"},
				{"p", "role:editor", "*", "config", "write"},
				{"p", "role:viewer", "*", "config", "read"},
				{"g", "dave", "role:editor", "*"},
				{"g", "eve", "role:viewer", "*"},
			},
			verify: func(t *testing.T, e *casbin.Enforcer) {
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

			ctrl := gomock.NewController(t)
			loader := casbin_mock.NewMockPolicyLoader(ctrl)
			loader.EXPECT().Load().Return(tt.rules, nil)

			e, err := casbin.NewEnforcer(loader)
			require.NoError(t, err)

			tt.verify(t, e)
		})
	}
}

func TestEnforcer_PolicyMethods(t *testing.T) {
	t.Parallel()

	t.Run("AddPolicy adds a rule and Enforce recognizes it", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)

		require.NoError(t, e.AddPolicy("role:custom", "ns1", "config", "read"))

		ok, err := e.Enforce("role:custom", "ns1", "config", "read")
		require.NoError(t, err)
		assert.True(t, ok, "custom policy should be enforced")
	})

	t.Run("RemovePolicy removes a rule and Enforce no longer recognizes it", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)

		require.NoError(t, e.AddPolicy("role:temp", "*", "namespace", "write"))

		ok, err := e.Enforce("role:temp", "*", "namespace", "write")
		require.NoError(t, err)
		require.True(t, ok, "policy must exist before removal")

		require.NoError(t, e.RemovePolicy("role:temp", "*", "namespace", "write"))

		ok, err = e.Enforce("role:temp", "*", "namespace", "write")
		require.NoError(t, err)
		assert.False(t, ok, "policy should not be enforced after removal")
	})

	t.Run("multiple AddRoleForUser calls do not duplicate", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)

		require.NoError(t, e.AddRoleForUser("frank", "role:editor", "*"))
		require.NoError(t, e.AddRoleForUser("frank", "role:editor", "*"))

		roles, err := e.GetRolesForUser("frank", "*")
		require.NoError(t, err)

		count := 0
		for _, r := range roles {
			if r == "role:editor" {
				count++
			}
		}
		assert.Equal(t, 1, count, "role:editor should appear exactly once for frank")
	})

	t.Run("GetRolesForUser returns correct roles after multiple assignments", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)

		require.NoError(t, e.AddRoleForUser("grace", "role:editor", "prod"))
		require.NoError(t, e.AddRoleForUser("grace", "role:viewer", "staging"))

		prodRoles, err := e.GetRolesForUser("grace", "prod")
		require.NoError(t, err)
		assert.Contains(t, prodRoles, "role:editor")
		assert.NotContains(t, prodRoles, "role:viewer")

		stagingRoles, err := e.GetRolesForUser("grace", "staging")
		require.NoError(t, err)
		assert.Contains(t, stagingRoles, "role:viewer")
		assert.NotContains(t, stagingRoles, "role:editor")
	})

	t.Run("GetRolesForUser returns empty slice for unknown user", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)

		roles, err := e.GetRolesForUser("nobody", "*")
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}
