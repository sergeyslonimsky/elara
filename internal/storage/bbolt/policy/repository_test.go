package policy_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/casbin/casbin/v2/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	policyrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/policy"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

//nolint:lll //casbin rule
const testCasbinModel = `
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && (r.dom == p.dom || p.dom == "*") && keyMatch(r.obj, p.obj) && (r.act == p.act || p.act == "*")
`

func newRepo(t *testing.T) (*policyrepo.Repository, pkgbbolt.Manager) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "elara.db")
	store, err := bbolt.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	mgr := pkgbbolt.NewManager(store.DB())

	return policyrepo.NewRepository(mgr), mgr
}

func newTestModel(t *testing.T) model.Model {
	t.Helper()

	m, err := model.NewModelFromString(testCasbinModel)
	require.NoError(t, err)

	return m
}

func TestRepository_LoadPolicy_Empty(t *testing.T) {
	t.Parallel()

	repo, _ := newRepo(t)

	m := newTestModel(t)
	require.NoError(t, repo.LoadPolicy(m))

	assert.Empty(t, m["p"]["p"].Policy)
}

func TestRepository_SaveAndLoad(t *testing.T) {
	t.Parallel()

	repo, _ := newRepo(t)

	m := newTestModel(t)
	require.NoError(t, m.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))
	require.NoError(t, m.AddPolicy("p", "p", []string{"reader", "*", "config", "read"}))
	require.NoError(t, m.AddPolicy("g", "g", []string{"alice", "admin", "*"}))
	require.NoError(t, repo.SavePolicy(m))

	m2 := newTestModel(t)
	require.NoError(t, repo.LoadPolicy(m2))

	pRules := m2["p"]["p"].Policy
	assert.Len(t, pRules, 2)

	gRules := m2["g"]["g"].Policy
	require.Len(t, gRules, 1)
	assert.Equal(t, []string{"alice", "admin", "*"}, gRules[0])
}

func TestRepository_SavePolicy_ClearAndRewrite(t *testing.T) {
	t.Parallel()

	repo, _ := newRepo(t)

	m := newTestModel(t)
	require.NoError(t, m.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))
	require.NoError(t, repo.SavePolicy(m))

	// Save a different model — old rules must be cleared.
	m2 := newTestModel(t)
	require.NoError(t, m2.AddPolicy("p", "p", []string{"reader", "*", "config", "read"}))
	require.NoError(t, repo.SavePolicy(m2))

	m3 := newTestModel(t)
	require.NoError(t, repo.LoadPolicy(m3))

	pRules := m3["p"]["p"].Policy
	require.Len(t, pRules, 1)
	assert.Equal(t, []string{"reader", "*", "config", "read"}, pRules[0])
}

func TestRepository_AddPolicy(t *testing.T) {
	t.Parallel()

	repo, _ := newRepo(t)

	require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))
	require.NoError(t, repo.AddPolicy("g", "g", []string{"alice", "admin", "*"}))

	m := newTestModel(t)
	require.NoError(t, repo.LoadPolicy(m))

	pRules := m["p"]["p"].Policy
	require.Len(t, pRules, 1)
	assert.Equal(t, []string{"admin", "*", "*", "*"}, pRules[0])

	gRules := m["g"]["g"].Policy
	require.Len(t, gRules, 1)
	assert.Equal(t, []string{"alice", "admin", "*"}, gRules[0])
}

func TestRepository_AddPolicy_Idempotent(t *testing.T) {
	t.Parallel()

	repo, _ := newRepo(t)

	require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))
	require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))

	m := newTestModel(t)
	require.NoError(t, repo.LoadPolicy(m))

	// bbolt Put is idempotent — same key overwrites itself, still one entry.
	assert.Len(t, m["p"]["p"].Policy, 1)
}

func TestRepository_RemovePolicy(t *testing.T) {
	t.Parallel()

	t.Run("removes existing rule", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))
		require.NoError(t, repo.AddPolicy("p", "p", []string{"reader", "*", "config", "read"}))

		require.NoError(t, repo.RemovePolicy("p", "p", []string{"admin", "*", "*", "*"}))

		m := newTestModel(t)
		require.NoError(t, repo.LoadPolicy(m))

		pRules := m["p"]["p"].Policy
		require.Len(t, pRules, 1)
		assert.Equal(t, []string{"reader", "*", "config", "read"}, pRules[0])
	})

	t.Run("removing non-existent rule is a no-op (no error)", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		// bbolt Delete on a missing key returns nil; verify the repo passes that through.
		require.NoError(t, repo.RemovePolicy("p", "p", []string{"ghost", "*", "*", "*"}))
	})
}

func TestRepository_RemoveFilteredPolicy(t *testing.T) {
	t.Parallel()

	t.Run("removes rules matching fieldIndex+values", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		require.NoError(t, repo.AddPolicy("g", "g", []string{"alice", "admin", "*"}))
		require.NoError(t, repo.AddPolicy("g", "g", []string{"bob", "reader", "*"}))
		require.NoError(t, repo.AddPolicy("g", "g", []string{"carol", "admin", "prod"}))

		require.NoError(t, repo.RemoveFilteredPolicy("g", "g", 0, "alice"))

		m := newTestModel(t)
		require.NoError(t, repo.LoadPolicy(m))

		gRules := m["g"]["g"].Policy
		assert.Len(t, gRules, 2)
		for _, r := range gRules {
			assert.NotEqual(t, "alice", r[0], "alice rule should have been removed")
		}
	})

	t.Run("filter with empty string in values is treated as wildcard for that position", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		require.NoError(t, repo.AddPolicy("g", "g", []string{"alice", "admin", "prod"}))
		require.NoError(t, repo.AddPolicy("g", "g", []string{"alice", "reader", "prod"}))
		require.NoError(t, repo.AddPolicy("g", "g", []string{"bob", "reader", "prod"}))

		// fieldIndex=0, values=["alice", ""] — match sub=alice, any role.
		require.NoError(t, repo.RemoveFilteredPolicy("g", "g", 0, "alice", ""))

		m := newTestModel(t)
		require.NoError(t, repo.LoadPolicy(m))

		gRules := m["g"]["g"].Policy
		require.Len(t, gRules, 1)
		assert.Equal(t, []string{"bob", "reader", "prod"}, gRules[0])
	})

	t.Run("no matches is a no-op", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		require.NoError(t, repo.AddPolicy("g", "g", []string{"alice", "admin", "*"}))
		require.NoError(t, repo.RemoveFilteredPolicy("g", "g", 0, "zoe"))

		m := newTestModel(t)
		require.NoError(t, repo.LoadPolicy(m))
		assert.Len(t, m["g"]["g"].Policy, 1)
	})
}

func TestRepository_CtxAware_JoinsOuterTx(t *testing.T) {
	t.Parallel()

	t.Run("AddPolicyCtx visible after WithTx commits", func(t *testing.T) {
		t.Parallel()
		repo, mgr := newRepo(t)
		ctx := t.Context()

		err := mgr.WithTx(ctx, func(ctx context.Context) error {
			if err := repo.AddPolicyCtx(ctx, "p", "p", []string{"admin", "*", "*", "*"}); err != nil {
				return err
			}

			return repo.AddPolicyCtx(ctx, "g", "g", []string{"alice", "admin", "*"})
		})
		require.NoError(t, err)

		m := newTestModel(t)
		require.NoError(t, repo.LoadPolicyCtx(t.Context(), m))
		assert.Len(t, m["p"]["p"].Policy, 1)
		assert.Len(t, m["g"]["g"].Policy, 1)
	})

	t.Run("AddPolicyCtx rolled back when WithTx errors", func(t *testing.T) {
		t.Parallel()
		repo, mgr := newRepo(t)
		ctx := t.Context()

		err := mgr.WithTx(ctx, func(ctx context.Context) error {
			if err := repo.AddPolicyCtx(ctx, "p", "p", []string{"admin", "*", "*", "*"}); err != nil {
				return err
			}

			return assert.AnError
		})
		require.ErrorIs(t, err, assert.AnError)

		m := newTestModel(t)
		require.NoError(t, repo.LoadPolicy(m))
		assert.Empty(t, m["p"]["p"].Policy)
	})

	t.Run("RemovePolicyCtx within outer tx", func(t *testing.T) {
		t.Parallel()
		repo, mgr := newRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))

		err := mgr.WithTx(ctx, func(ctx context.Context) error {
			return repo.RemovePolicyCtx(ctx, "p", "p", []string{"admin", "*", "*", "*"})
		})
		require.NoError(t, err)

		m := newTestModel(t)
		require.NoError(t, repo.LoadPolicy(m))
		assert.Empty(t, m["p"]["p"].Policy)
	})

	t.Run("RemoveFilteredPolicyCtx rollback discards deletes", func(t *testing.T) {
		t.Parallel()
		repo, mgr := newRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.AddPolicy("g", "g", []string{"alice", "admin", "*"}))
		require.NoError(t, repo.AddPolicy("g", "g", []string{"bob", "reader", "*"}))

		err := mgr.WithTx(ctx, func(ctx context.Context) error {
			if err := repo.RemoveFilteredPolicyCtx(ctx, "g", "g", 0, "alice"); err != nil {
				return err
			}

			return assert.AnError
		})
		require.ErrorIs(t, err, assert.AnError)

		m := newTestModel(t)
		require.NoError(t, repo.LoadPolicy(m))
		// Both rules still present after rollback.
		assert.Len(t, m["g"]["g"].Policy, 2)
	})
}

func TestRepository_ListPermissionsForSubject(t *testing.T) {
	t.Parallel()

	t.Run("returns p-rules matching subject", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))
		require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "ns1", "config", "read"}))
		require.NoError(t, repo.AddPolicy("p", "p", []string{"reader", "*", "config", "read"}))
		// g-rules must be ignored.
		require.NoError(t, repo.AddPolicy("g", "g", []string{"admin", "admin", "*"}))

		got, err := repo.ListPermissionsForSubject(ctx, "admin")
		require.NoError(t, err)
		assert.Len(t, got, 2)
		for _, r := range got {
			assert.Equal(t, "admin", r[0])
		}
	})

	t.Run("empty when no match", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)
		ctx := t.Context()

		require.NoError(t, repo.AddPolicy("p", "p", []string{"admin", "*", "*", "*"}))

		got, err := repo.ListPermissionsForSubject(ctx, "ghost")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("empty store returns empty", func(t *testing.T) {
		t.Parallel()
		repo, _ := newRepo(t)

		got, err := repo.ListPermissionsForSubject(t.Context(), "admin")
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
