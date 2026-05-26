package auth_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

// TestBootstrap_PDP_FreshDB_RequiresReload reproduces the production wiring
// order — enforcer built BEFORE bootstrap on a fresh bbolt store — and locks
// in the contract that the caller must call enforcer.LoadPolicy() after
// AdminBootstrap to make the seeded rules visible to the in-memory model.
//
// Regression: skipping the post-bootstrap reload caused
// pdp.ListPermissions(admin) to return [] on first boot. The Web UI builds
// CASL from these permissions, so an empty list collapses the sidebar to
// Dashboard-only — basic-auth admin appeared to have no access.
// See cmd/service/main.go: bootstrap → svc.Enforcer.LoadPolicy().
func TestBootstrap_PDP_FreshDB_RequiresReload(t *testing.T) {
	t.Parallel()

	store, err := bbolt.Open(filepath.Join(t.TempDir(), "bootstrap_pdp.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	users := bbolt.NewUserRepo(store)
	groups := bbolt.NewGroupRepo(store)
	policies := bbolt.NewPolicyRepo(store)
	txm := bbolt.NewTxManager(store.DB())

	// Production order: enforcer is constructed BEFORE bootstrap runs.
	// On a fresh DB it loads only the built-in role policies.
	enforcer, err := casbin.NewEnforcer(policies)
	require.NoError(t, err)

	bs := auth.NewAdminBootstrap(txm, users, groups, policies)
	const username = "superadmin@example.com"
	require.NoError(t, bs.BootstrapBasic(t.Context(), username, "initial-password"))

	pdp := authz.NewPDP(enforcer)

	// Without the reload, the bootstrap-seeded (group:system:superadmin, *, *, *)
	// p-rule and the admin→group g-rule live in bbolt only — the in-memory
	// model is stale. ListPermissions returns nothing.
	beforeReload, err := pdp.ListPermissions(username)
	require.NoError(t, err)
	assert.Empty(t, beforeReload, "stale cache must not surface the wildcard before reload")

	// Match production: caller reloads policy from bbolt post-bootstrap.
	require.NoError(t, enforcer.LoadPolicy())

	afterReload, err := pdp.ListPermissions(username)
	require.NoError(t, err)
	wildcard := domain.Permission{
		Object: domain.ObjectAll,
		Action: domain.ActionAll,
		Domain: domain.DomainAll,
	}
	assert.Contains(t, afterReload, wildcard,
		"after LoadPolicy, the admin must inherit (*,*,*) from the superadmin group")
}
