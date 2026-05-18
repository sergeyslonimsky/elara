package casbin_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// newTestEnforcer creates an Enforcer backed by a real bbolt PolicyRepo in a
// temp directory. When rules is non-empty it pre-seeds them through PolicyRepo
// before constructing the enforcer, so the seed-on-empty branch is skipped.
// Each rule is [ptype, fields...] (e.g. {"p", "alice", "*", "config", "read"}).
func newTestEnforcer(t *testing.T, rules [][]string) *casbin.Enforcer {
	t.Helper()

	e, _ := newTestEnforcerWithTxM(t, rules)

	return e
}

// newTestEnforcerWithTxM constructs an Enforcer + TxManager backed by a real
// bbolt store. Pre-seeds raw rules through PolicyRepo (bypassing the enforcer)
// before NewEnforcer runs, so seed-on-empty is skipped when rules is non-empty.
func newTestEnforcerWithTxM(t *testing.T, rules [][]string) (*casbin.Enforcer, storage.TxManager) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "casbin.db")

	store, err := bbolt.Open(path)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	policies := bbolt.NewPolicyRepo(store)

	for _, rule := range rules {
		if len(rule) < 2 {
			continue
		}

		ptype := rule[0]

		sec := "p"
		if ptype == "g" || ptype == "g2" || ptype == "g3" {
			sec = "g"
		}

		require.NoError(t, policies.AddPolicy(sec, ptype, rule[1:]))
	}

	e, err := casbin.NewEnforcer(policies)
	require.NoError(t, err)

	return e, bbolt.NewTxManager(store.DB())
}

// seedRole adds a g-rule (role assignment) through a real WriteTx so persistence
// and cache stay in sync — equivalent to the old Enforcer.AddRoleForUser bridge.
func seedRole(t *testing.T, e *casbin.Enforcer, txm storage.TxManager, user, role, dom string) {
	t.Helper()
	require.NoError(t, e.WriteTx(t.Context(), txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.AddRoleForUser(user, role, dom)
	}))
}

// removeRole removes a g-rule through a real WriteTx.
func removeRole(t *testing.T, e *casbin.Enforcer, txm storage.TxManager, user, role, dom string) {
	t.Helper()
	require.NoError(t, e.WriteTx(t.Context(), txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.RemoveRoleForUser(user, role, dom)
	}))
}

// seedPolicy adds a p-rule through a real WriteTx.
func seedPolicy(t *testing.T, e *casbin.Enforcer, txm storage.TxManager, sub, dom, obj, act string) {
	t.Helper()
	require.NoError(t, e.WriteTx(t.Context(), txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.AddPolicy(sub, dom, obj, act)
	}))
}

// removePolicy removes a p-rule through a real WriteTx.
func removePolicy(t *testing.T, e *casbin.Enforcer, txm storage.TxManager, sub, dom, obj, act string) {
	t.Helper()
	require.NoError(t, e.WriteTx(t.Context(), txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		return txe.RemovePolicy(sub, dom, obj, act)
	}))
}

func TestNewEnforcer_SeedsBuiltinPoliciesOnEmpty(t *testing.T) {
	t.Parallel()

	e := newTestEnforcer(t, nil)

	rules := e.GetPolicy()
	assert.Len(t, rules, 16, "expected 16 built-in p rules seeded into empty storage")
}

func TestEnforce_AdminCanDoAnything(t *testing.T) {
	t.Parallel()

	e, txm := newTestEnforcerWithTxM(t, nil)

	seedRole(t, e, txm, "alice", "admin", "*")

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

	e, txm := newTestEnforcerWithTxM(t, nil)

	seedRole(t, e, txm, "bob", "reader", "*")

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

	e, txm := newTestEnforcerWithTxM(t, nil)

	seedRole(t, e, txm, "carol", "writer", "*")

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

	e, txm := newTestEnforcerWithTxM(t, nil)

	// dave has role:viewer only in domain "prod"
	seedRole(t, e, txm, "dave", "reader", "prod")

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

	e, txm := newTestEnforcerWithTxM(t, nil)

	seedRole(t, e, txm, "eve", "writer", "*")

	ok, err := e.Enforce("eve", "*", "namespace", "read")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestRemoveRoleForUser(t *testing.T) {
	t.Parallel()

	e, txm := newTestEnforcerWithTxM(t, nil)

	seedRole(t, e, txm, "frank", "writer", "*")

	ok, err := e.Enforce("frank", "*", "config", "write")
	require.NoError(t, err)
	require.True(t, ok)

	removeRole(t, e, txm, "frank", "writer", "*")

	ok, err = e.Enforce("frank", "*", "config", "write")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestEnforcer_GetRolesForUser(t *testing.T) {
	t.Parallel()

	e, txm := newTestEnforcerWithTxM(t, nil)

	seedRole(t, e, txm, "alice", "admin", "*")

	roles, err := e.GetRolesForUser("alice", "*")
	require.NoError(t, err)
	assert.Contains(t, roles, "admin")
}

func TestEnforcer_Methods(t *testing.T) { // NOSONAR
	t.Parallel()

	t.Run("GetAllRoles returns roles in domain", func(t *testing.T) {
		t.Parallel()

		e, txm := newTestEnforcerWithTxM(t, nil)
		seedRole(t, e, txm, "alice", "admin", "*")
		seedRole(t, e, txm, "bob", "reader", "*")

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

		e, txm := newTestEnforcerWithTxM(t, nil)
		seedRole(t, e, txm, "grace", "reader", "ns1")

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

		e, txm := newTestEnforcerWithTxM(t, nil)
		seedPolicy(t, e, txm, "role:custom", "*", "config", "read")

		rules := e.GetPolicy()
		found := false
		for _, r := range rules {
			if r[0] == "role:custom" {
				found = true

				break
			}
		}
		require.True(t, found, "policy should exist before removal")

		removePolicy(t, e, txm, "role:custom", "*", "config", "read")

		rules = e.GetPolicy()
		for _, r := range rules {
			if r[0] == "role:custom" {
				t.Error("policy should have been removed")
			}
		}
	})

	t.Run("RemoveRoleForUser removes g rule", func(t *testing.T) {
		t.Parallel()

		e, txm := newTestEnforcerWithTxM(t, nil)
		seedRole(t, e, txm, "ivan", "writer", "*")

		ok, err := e.Enforce("ivan", "*", "config", "write")
		require.NoError(t, err)
		require.True(t, ok, "ivan should be able to write before role removal")

		removeRole(t, e, txm, "ivan", "writer", "*")

		ok, err = e.Enforce("ivan", "*", "config", "write")
		require.NoError(t, err)
		assert.False(t, ok, "ivan should no longer be able to write after role removal")
	})
}
