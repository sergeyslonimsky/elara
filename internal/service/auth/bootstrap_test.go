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
)

// newBootstrapFixture wires a fresh bbolt store + repositories needed to
// exercise AdminBootstrap end-to-end. Returns the bootstrap helper plus the
// repos used by assertions.
func newBootstrapFixture(t *testing.T) (
	*auth.AdminBootstrap,
	*bbolt.UserRepo,
	*bbolt.GroupRepo,
	*bbolt.PolicyRepo,
) {
	t.Helper()

	store, err := bbolt.Open(filepath.Join(t.TempDir(), "bootstrap.db"))
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	users := bbolt.NewUserRepo(store)
	groups := bbolt.NewGroupRepo(store)
	policies := bbolt.NewPolicyRepo(store)
	txm := bbolt.NewTxManager(store.DB())

	return auth.NewAdminBootstrap(txm, users, groups, policies), users, groups, policies
}

// TestAdminBootstrap_Idempotent verifies the architecture §11 invariant:
// running Bootstrap twice on the same store must succeed without duplicating
// records — group/user/g-rule/p-rule are all keyed and re-asserted in place.
func TestAdminBootstrap_Idempotent(t *testing.T) {
	t.Parallel()

	bs, users, groups, policies := newBootstrapFixture(t)
	ctx := t.Context()

	username := "superadmin@example.com"

	// First run — seeds everything from empty state.
	require.NoError(t, bs.BootstrapBasic(ctx, username, "initial-password"))

	// Second run on the populated store — must not error and must not duplicate.
	require.NoError(t, bs.BootstrapBasic(ctx, username, "initial-password"))

	// Group: exactly one system:superadmin, System=true. Membership is
	// stored as a Casbin g-rule (not in bbolt) and is verified through the
	// AddPolicy idempotence contract — bbolt only carries entity metadata
	// since the SSoT refactor.
	grp, err := groups.FindByName(ctx, domain.SystemGroupSuperAdmin)
	require.NoError(t, err)
	assert.True(t, grp.System, "superadmin group must have System=true")

	// User: exactly one superadmin, System=true.
	user, err := users.Get(ctx, username)
	require.NoError(t, err)
	assert.True(t, user.System, "superadmin user must have System=true")

	// p-rule: superadmin has (*, *, *) wildcard. ListPermissionsForSubject returns
	// rule rows of shape [dom, obj, act] (subject is implicit in the query).
	perms, err := policies.ListPermissionsForSubject(casbin.GroupSubject(domain.SystemGroupSuperAdmin))
	require.NoError(t, err)

	wildcardCount := 0
	for _, r := range perms {
		// ListPermissionsForSubject returns [sub, dom, obj, act].
		if len(r) >= 4 && r[1] == domain.DomainAll && r[2] == string(domain.ObjectAll) &&
			r[3] == string(domain.ActionAll) {
			wildcardCount++
		}
	}
	assert.Equal(t, 1, wildcardCount, "exactly one (*, *, *) p-rule for superadmin group")
}

// TestAdminBootstrap_RecoversRemovedPolicy verifies the architecture §11
// break-glass guarantee: if the wildcard p-rule is removed from storage (e.g.
// via direct adapter mutation), re-running Bootstrap restores it. This protects
// against lockout from corrupted Casbin policy state.
func TestAdminBootstrap_RecoversRemovedPolicy(t *testing.T) {
	t.Parallel()

	bs, _, _, policies := newBootstrapFixture(t)
	ctx := t.Context()

	require.NoError(t, bs.BootstrapBasic(ctx, "superadmin@example.com", "pw"))

	// Tamper: remove the wildcard p-rule.
	wildcardRule := []string{
		casbin.GroupSubject(domain.SystemGroupSuperAdmin),
		domain.DomainAll,
		string(domain.ObjectAll),
		string(domain.ActionAll),
	}
	require.NoError(t, policies.RemovePolicy("p", "p", wildcardRule))

	// Re-run bootstrap — break-glass should restore the missing rule.
	require.NoError(t, bs.BootstrapBasic(ctx, "superadmin@example.com", "pw"))

	perms, err := policies.ListPermissionsForSubject(casbin.GroupSubject(domain.SystemGroupSuperAdmin))
	require.NoError(t, err)

	found := false
	for _, r := range perms {
		// ListPermissionsForSubject returns [sub, dom, obj, act].
		if len(r) >= 4 && r[1] == domain.DomainAll && r[2] == string(domain.ObjectAll) &&
			r[3] == string(domain.ActionAll) {
			found = true

			break
		}
	}
	assert.True(t, found, "Bootstrap re-run must restore the superadmin (*,*,*) p-rule")
}
