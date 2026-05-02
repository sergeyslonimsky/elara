package casbin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
)

type mockLoader struct {
	rules [][]string
}

func (m *mockLoader) Load() ([][]string, error) { return m.rules, nil }
func (m *mockLoader) Save(rules [][]string) error {
	m.rules = rules

	return nil
}

func newTestEnforcer(t *testing.T, rules [][]string) *casbin.Enforcer {
	t.Helper()

	loader := &mockLoader{rules: rules}

	e, err := casbin.NewEnforcer(loader)
	require.NoError(t, err)

	return e
}

func TestNewEnforcer_SeedsBuiltinPoliciesOnEmpty(t *testing.T) {
	t.Parallel()

	loader := &mockLoader{}
	_, err := casbin.NewEnforcer(loader)
	require.NoError(t, err)

	assert.NotEmpty(t, loader.rules, "built-in policies should be saved when storage is empty")

	var pRules [][]string

	for _, r := range loader.rules {
		if len(r) > 0 && r[0] == "p" {
			pRules = append(pRules, r)
		}
	}

	assert.Len(t, pRules, 6, "expected 6 built-in p rules")
}

func TestEnforce_AdminCanDoAnything(t *testing.T) {
	t.Parallel()

	e := newTestEnforcer(t, nil)

	require.NoError(t, e.AddRoleForUser("alice", "role:admin", "*"))

	tests := []struct {
		name   string
		obj    string
		act    string
		domain string
	}{
		{name: "read config", domain: "*", obj: "config", act: "read"},
		{name: "write config", domain: "*", obj: "config", act: "write"},
		{name: "delete anything", domain: "*", obj: "anything", act: "delete"},
		{name: "manage namespace", domain: "*", obj: "namespace", act: "write"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ok, err := e.Enforce("alice", tc.domain, tc.obj, tc.act)
			require.NoError(t, err)
			assert.True(t, ok)
		})
	}
}

func TestEnforce_ViewerCanReadConfigButNotWrite(t *testing.T) {
	t.Parallel()

	e := newTestEnforcer(t, nil)

	require.NoError(t, e.AddRoleForUser("bob", "role:viewer", "*"))

	t.Run("read config allowed", func(t *testing.T) {
		t.Parallel()

		ok, err := e.Enforce("bob", "*", "config", "read")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("write config denied", func(t *testing.T) {
		t.Parallel()

		ok, err := e.Enforce("bob", "*", "config", "write")
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestEnforce_EditorCanReadAndWriteConfig(t *testing.T) {
	t.Parallel()

	e := newTestEnforcer(t, nil)

	require.NoError(t, e.AddRoleForUser("carol", "role:editor", "*"))

	tests := []struct {
		name    string
		act     string
		allowed bool
	}{
		{name: "read config", act: "read", allowed: true},
		{name: "write config", act: "write", allowed: true},
		{name: "delete config", act: "delete", allowed: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ok, err := e.Enforce("carol", "*", "config", tc.act)
			require.NoError(t, err)
			assert.Equal(t, tc.allowed, ok)
		})
	}
}

func TestEnforce_NamespaceScoping(t *testing.T) {
	t.Parallel()

	e := newTestEnforcer(t, nil)

	// dave has role:viewer only in domain "prod"
	require.NoError(t, e.AddRoleForUser("dave", "role:viewer", "prod"))

	t.Run("can read config in prod", func(t *testing.T) {
		t.Parallel()

		ok, err := e.Enforce("dave", "prod", "config", "read")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("cannot read config in dev", func(t *testing.T) {
		t.Parallel()

		ok, err := e.Enforce("dave", "dev", "config", "read")
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestAddRoleForUser_ThenEnforce(t *testing.T) {
	t.Parallel()

	e := newTestEnforcer(t, nil)

	require.NoError(t, e.AddRoleForUser("eve", "role:editor", "*"))

	ok, err := e.Enforce("eve", "*", "namespace", "read")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestRemoveRoleForUser(t *testing.T) {
	t.Parallel()

	e := newTestEnforcer(t, nil)

	require.NoError(t, e.AddRoleForUser("frank", "role:editor", "*"))

	ok, err := e.Enforce("frank", "*", "config", "write")
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, e.RemoveRoleForUser("frank", "role:editor", "*"))

	ok, err = e.Enforce("frank", "*", "config", "write")
	require.NoError(t, err)
	assert.False(t, ok)
}
