package casbin_test

import (
	"testing"

	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
	casbin_mock "github.com/sergeyslonimsky/elara/internal/auth/casbin/mocks"
)

// newTestEnforcer creates an Enforcer seeded with built-in policies.
// When rules is nil/empty the adapter will seed built-ins and persist them.
// When rules is non-nil the adapter loads them via LoadPolicy (simulating pre-existing policy).
func newTestEnforcer(t *testing.T, rules [][]string) *casbin.Enforcer {
	t.Helper()

	ctrl := gomock.NewController(t)
	adapter := casbin_mock.NewMockAdapter(ctrl)

	if len(rules) == 0 {
		// Empty storage: enforcer will seed built-ins and call SavePolicy once.
		adapter.EXPECT().LoadPolicy(gomock.Any()).Return(nil)
		adapter.EXPECT().SavePolicy(gomock.Any()).Return(nil)
	} else {
		// Pre-existing rules: populate the model inside LoadPolicy.
		adapter.EXPECT().LoadPolicy(gomock.Any()).DoAndReturn(func(m casbinmodel.Model) error {
			for _, rule := range rules {
				if len(rule) == 0 {
					continue
				}

				require.NoError(t, persist.LoadPolicyArray(rule, m))
			}

			return nil
		})
	}

	// AutoSave will call AddPolicy/RemovePolicy on the adapter for runtime mutations.
	adapter.EXPECT().AddPolicy(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	adapter.EXPECT().RemovePolicy(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	adapter.EXPECT().RemoveFilteredPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	e, err := casbin.NewEnforcer(adapter)
	require.NoError(t, err)

	return e
}

func TestNewEnforcer_SeedsBuiltinPoliciesOnEmpty(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	adapter := casbin_mock.NewMockAdapter(ctrl)

	var savedModel casbinmodel.Model
	adapter.EXPECT().LoadPolicy(gomock.Any()).Return(nil)
	adapter.EXPECT().SavePolicy(gomock.Any()).DoAndReturn(func(m casbinmodel.Model) error {
		savedModel = m

		return nil
	})
	adapter.EXPECT().AddPolicy(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	adapter.EXPECT().RemovePolicy(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	adapter.EXPECT().RemoveFilteredPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	_, err := casbin.NewEnforcer(adapter)
	require.NoError(t, err)

	assert.NotNil(t, savedModel, "built-in policies should be saved when storage is empty")

	pAssert, hasPType := savedModel["p"]["p"]
	require.True(t, hasPType, "expected 'p' policy type in saved model")
	assert.Len(t, pAssert.Policy, 16, "expected 16 built-in p rules")
}

func TestEnforce_AdminCanDoAnything(t *testing.T) {
	t.Parallel()

	e := newTestEnforcer(t, nil)

	require.NoError(t, e.AddRoleForUser("alice", "admin", "*"))

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

	require.NoError(t, e.AddRoleForUser("bob", "reader", "*"))

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

	require.NoError(t, e.AddRoleForUser("carol", "writer", "*"))

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
	require.NoError(t, e.AddRoleForUser("dave", "reader", "prod"))

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

	require.NoError(t, e.AddRoleForUser("eve", "writer", "*"))

	ok, err := e.Enforce("eve", "*", "namespace", "read")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestRemoveRoleForUser(t *testing.T) {
	t.Parallel()

	e := newTestEnforcer(t, nil)

	require.NoError(t, e.AddRoleForUser("frank", "writer", "*"))

	ok, err := e.Enforce("frank", "*", "config", "write")
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, e.RemoveRoleForUser("frank", "writer", "*"))

	ok, err = e.Enforce("frank", "*", "config", "write")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestEnforcer_GetRolesForUser(t *testing.T) {
	t.Parallel()

	e := newTestEnforcer(t, nil)

	require.NoError(t, e.AddRoleForUser("alice", "admin", "*"))

	roles, err := e.GetRolesForUser("alice", "*")
	require.NoError(t, err)
	assert.Contains(t, roles, "admin")
}

func TestEnforcer_SeedPassthroughAdmin(t *testing.T) {
	t.Parallel()

	e := newTestEnforcer(t, nil)

	require.NoError(t, e.SeedPassthroughAdmin())

	ok, err := e.Enforce("local-admin@elara.internal", "*", "config", "read")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestEnforcer_Methods(t *testing.T) { // NOSONAR
	t.Parallel()

	t.Run("GetAllRoles returns roles in domain", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)
		require.NoError(t, e.AddRoleForUser("alice", "admin", "*"))
		require.NoError(t, e.AddRoleForUser("bob", "reader", "*"))

		roles, err := e.GetAllRoles("*")
		require.NoError(t, err)
		assert.Contains(t, roles, "admin")
		assert.Contains(t, roles, "reader")
	})

	t.Run("GetPolicy returns builtin p rules", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)
		rules := e.GetPolicy()
		assert.NotEmpty(t, rules, "built-in p rules should be present after init")
		assert.Len(t, rules, 16, "expected 16 built-in p rules")
	})

	t.Run("GetGroupingPolicy returns added g rules", func(t *testing.T) {
		t.Parallel()

		e := newTestEnforcer(t, nil)
		require.NoError(t, e.AddRoleForUser("grace", "reader", "ns1"))

		gRules := e.GetGroupingPolicy()
		assert.NotEmpty(t, gRules)

		found := false
		for _, r := range gRules {
			if len(r) >= 3 && r[0] == "grace" && r[1] == "reader" && r[2] == "ns1" {
				found = true

				break
			}
		}

		assert.True(t, found, "expected g rule for grace not found")
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
		require.NoError(t, e.AddRoleForUser("ivan", "writer", "*"))

		ok, err := e.Enforce("ivan", "*", "config", "write")
		require.NoError(t, err)
		require.True(t, ok, "ivan should be able to write before role removal")

		require.NoError(t, e.RemoveRoleForUser("ivan", "writer", "*"))

		ok, err = e.Enforce("ivan", "*", "config", "write")
		require.NoError(t, err)
		assert.False(t, ok, "ivan should no longer be able to write after role removal")
	})
}
