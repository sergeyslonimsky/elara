package casbin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
	casbin_mock "github.com/sergeyslonimsky/elara/internal/auth/casbin/mocks"
)

// newTestEnforcer creates an Enforcer seeded with built-in policies.
// When rules is nil/empty the loader will seed built-ins and persist them.
// When rules is non-nil the loader returns them directly (pre-existing policy).
func newTestEnforcer(t *testing.T, rules [][]string) *casbin.Enforcer {
	t.Helper()

	ctrl := gomock.NewController(t)
	loader := casbin_mock.NewMockPolicyLoader(ctrl)

	if len(rules) == 0 {
		// Empty storage: enforcer will seed built-ins and call Save once.
		loader.EXPECT().Load().Return(nil, nil)
		loader.EXPECT().Save(gomock.Any()).Return(nil)
	} else {
		loader.EXPECT().Load().Return(rules, nil)
	}

	e, err := casbin.NewEnforcer(loader)
	require.NoError(t, err)

	return e
}

func TestNewEnforcer_SeedsBuiltinPoliciesOnEmpty(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	loader := casbin_mock.NewMockPolicyLoader(ctrl)

	var savedRules [][]string
	loader.EXPECT().Load().Return(nil, nil)
	loader.EXPECT().Save(gomock.Any()).DoAndReturn(func(rules [][]string) error {
		savedRules = rules

		return nil
	})

	_, err := casbin.NewEnforcer(loader)
	require.NoError(t, err)

	assert.NotEmpty(t, savedRules, "built-in policies should be saved when storage is empty")

	var pRules [][]string

	for _, r := range savedRules {
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

func TestEnforcer_Methods(t *testing.T) {
	t.Parallel()

	t.Run("GetAllRoles returns roles in domain", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)
		require.NoError(t, e.AddRoleForUser("alice", "role:admin", "*"))
		require.NoError(t, e.AddRoleForUser("bob", "role:viewer", "*"))

		roles, err := e.GetAllRoles("*")
		require.NoError(t, err)
		assert.Contains(t, roles, "role:admin")
		assert.Contains(t, roles, "role:viewer")
	})

	t.Run("GetPolicy returns builtin p rules", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)
		rules := e.GetPolicy()
		assert.NotEmpty(t, rules, "built-in p rules should be present after init")
		assert.Len(t, rules, 6, "expected 6 built-in p rules")
	})

	t.Run("GetGroupingPolicy returns added g rules", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)
		require.NoError(t, e.AddRoleForUser("grace", "role:viewer", "ns1"))

		gRules := e.GetGroupingPolicy()
		assert.NotEmpty(t, gRules)

		found := false
		for _, r := range gRules {
			if len(r) >= 3 && r[0] == "grace" && r[1] == "role:viewer" && r[2] == "ns1" {
				found = true

				break
			}
		}

		assert.True(t, found, "expected g rule for grace not found")
	})

	t.Run("SavePolicy calls loader.Save with all rules", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		saveLoader := casbin_mock.NewMockPolicyLoader(ctrl)

		// Initial creation (empty storage) uses its own mock.
		e := newTestEnforcer(t, nil)

		require.NoError(t, e.AddRoleForUser("henry", "role:admin", "*"))

		var capturedRules [][]string
		saveLoader.EXPECT().Save(gomock.Any()).DoAndReturn(func(rules [][]string) error {
			capturedRules = rules

			return nil
		})

		require.NoError(t, e.SavePolicy(saveLoader))

		var pCount, gCount int
		for _, r := range capturedRules {
			if len(r) > 0 && r[0] == "p" {
				pCount++
			}
			if len(r) > 0 && r[0] == "g" {
				gCount++
			}
		}
		assert.Equal(t, 6, pCount, "expected 6 p rules in saved output")
		assert.Equal(t, 1, gCount, "expected 1 g rule for henry in saved output")
	})

	t.Run("RemovePolicy removes a p rule", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)
		require.NoError(t, e.AddPolicy("role:custom", "*", "config", "read"))

		rules := e.GetPolicy()
		found := false
		for _, r := range rules {
			if r[0] == "role:custom" {
				found = true

				break
			}
		}
		require.True(t, found, "policy should exist before removal")

		require.NoError(t, e.RemovePolicy("role:custom", "*", "config", "read"))

		rules = e.GetPolicy()
		for _, r := range rules {
			if r[0] == "role:custom" {
				t.Error("policy should have been removed")
			}
		}
	})

	t.Run("RemoveRoleForUser removes g rule", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)
		require.NoError(t, e.AddRoleForUser("ivan", "role:editor", "*"))

		ok, err := e.Enforce("ivan", "*", "config", "write")
		require.NoError(t, err)
		require.True(t, ok, "ivan should be able to write before role removal")

		require.NoError(t, e.RemoveRoleForUser("ivan", "role:editor", "*"))

		ok, err = e.Enforce("ivan", "*", "config", "write")
		require.NoError(t, err)
		assert.False(t, ok, "ivan should no longer be able to write after role removal")
	})
}
