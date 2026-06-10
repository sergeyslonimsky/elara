package auth_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/storage/bbolt"
	grouprepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/group"
	policyrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/policy"
	userrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/user"
	pkgbbolt "github.com/sergeyslonimsky/elara/pkg/bbolt"
)

// newBootstrapFixture wires a fresh bbolt store + repositories needed to
// exercise AdminBootstrap end-to-end. Returns the bootstrap helper plus the
// repos used by assertions.
func newBootstrapFixture(t *testing.T) (
	*auth.AdminBootstrap,
	*userrepo.Repository,
	*grouprepo.Repository,
	*policyrepo.Repository,
) {
	t.Helper()

	store, err := bbolt.Open(filepath.Join(t.TempDir(), "bootstrap.db"))
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	storageManager := bbolt.NewManager(store.DB())
	pkgManager := pkgbbolt.NewManager(store.DB())
	users := userrepo.NewRepository(pkgManager)
	groups := grouprepo.NewRepository(pkgManager)
	policies := policyrepo.NewRepository(pkgManager)

	userSvc := auth.NewUserService(users)

	return auth.NewAdminBootstrap(storageManager, userSvc, groups, policies), users, groups, policies
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
	grp, err := groups.Get(ctx, domain.SystemGroupSuperAdmin)
	require.NoError(t, err)
	assert.True(t, grp.System, "superadmin group must have System=true")

	// User: exactly one superadmin, System=true.
	user, err := users.GetByIdentity(ctx, string(domain.ProviderBasic), username)
	require.NoError(t, err)
	assert.True(t, user.System, "superadmin user must have System=true")

	// p-rule: superadmin has (*, *, *) wildcard. ListPermissionsForSubject returns
	// rule rows of shape [dom, obj, act] (subject is implicit in the query).
	perms, err := policies.ListPermissionsForSubject(
		ctx,
		domain.GroupResource(domain.SystemGroupSuperAdmin),
	)
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

// TestAdminBootstrap_OIDC_CreatesPlaceholderUser is the regression for the
// EL-OIDC bootstrap bug: BootstrapOIDC used to skip user creation, leaving the
// first OIDC login with nothing to email-fallback into — Callback rejected it
// with ErrIdentityNotProvisioned. Now the placeholder must exist with
// System=true, Email=adminEmail, and empty Identities so the first matching
// (provider, sub) lands cleanly via linkOIDCByEmail.
func TestAdminBootstrap_OIDC_CreatesPlaceholderUser(t *testing.T) {
	t.Parallel()

	bs, users, groups, _ := newBootstrapFixture(t)
	ctx := t.Context()

	adminEmail := "admin@example.com"

	// First run seeds everything from empty state.
	require.NoError(t, bs.BootstrapOIDC(ctx, adminEmail))

	// Second run on the populated store — idempotent (no duplicates, no errors).
	require.NoError(t, bs.BootstrapOIDC(ctx, adminEmail))

	grp, err := groups.Get(ctx, domain.SystemGroupSuperAdmin)
	require.NoError(t, err)
	assert.True(t, grp.System, "superadmin group must have System=true")

	user, err := users.GetSystemUser(ctx)
	require.NoError(t, err)
	assert.True(t, user.System, "placeholder must be System=true")
	assert.Equal(t, adminEmail, user.Email)
	assert.Equal(t, domain.UserStatusActive, user.Status)
	assert.Empty(t,
		user.Identities,
		"placeholder must have empty Identities so first OIDC login can attach (provider, sub) via linkOIDCByEmail",
	)
}

// TestAdminBootstrap_OIDC_SyncsRotatedEmail verifies that rotating
// oidc.adminEmail in config and restarting Elara updates the placeholder's
// Email in place via BootstrapSync, rather than failing or creating a duplicate
// system user.
func TestAdminBootstrap_OIDC_SyncsRotatedEmail(t *testing.T) {
	t.Parallel()

	bs, users, _, _ := newBootstrapFixture(t)
	ctx := t.Context()

	initial := "old-admin@example.com"
	rotated := "new-admin@example.com"

	require.NoError(t, bs.BootstrapOIDC(ctx, initial))
	require.NoError(t, bs.BootstrapOIDC(ctx, rotated))

	user, err := users.GetSystemUser(ctx)
	require.NoError(t, err)
	assert.Equal(t, rotated, user.Email, "rotated adminEmail must be synced into the existing placeholder")
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
		domain.GroupResource(domain.SystemGroupSuperAdmin),
		domain.DomainAll,
		string(domain.ObjectAll),
		string(domain.ActionAll),
	}
	require.NoError(t, policies.RemovePolicyCtx(ctx, "p", "p", wildcardRule))

	// Re-run bootstrap — break-glass should restore the missing rule.
	require.NoError(t, bs.BootstrapBasic(ctx, "superadmin@example.com", "pw"))

	perms, err := policies.ListPermissionsForSubject(
		ctx,
		domain.GroupResource(domain.SystemGroupSuperAdmin),
	)
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
