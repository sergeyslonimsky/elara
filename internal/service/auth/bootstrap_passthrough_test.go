package auth_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// TestAdminBootstrap_Passthrough_Idempotent verifies BootstrapPassthrough
// seeds the synthetic local-admin user + superadmin group/policy and that a
// second run does not duplicate or error, mirroring the Basic/OIDC
// idempotency invariant.
func TestAdminBootstrap_Passthrough_Idempotent(t *testing.T) {
	t.Parallel()

	bs, users, groups, _ := newBootstrapFixture(t)
	ctx := t.Context()

	require.NoError(t, bs.BootstrapPassthrough(ctx))
	require.NoError(t, bs.BootstrapPassthrough(ctx))

	grp, err := groups.Get(ctx, domain.SystemGroupSuperAdmin)
	require.NoError(t, err)
	assert.True(t, grp.System, "superadmin group must have System=true")

	user, err := users.GetByIdentity(ctx, string(domain.ProviderBasic), auth.PassthroughEmail)
	require.NoError(t, err)
	assert.True(t, user.System, "passthrough user must have System=true")
	assert.Equal(t, auth.PassthroughEmail, user.Email)
	assert.Equal(t, domain.UserStatusActive, user.Status)
	require.Len(t, user.Identities, 1)
	assert.Equal(t, domain.ProviderBasic, user.Identities[0].Provider)
}

// TestAdminBootstrap_EnsureMember verifies EnsureMember adds an
// already-provisioned user to the superadmin group, and fails when the
// group has not been bootstrapped yet.
func TestAdminBootstrap_EnsureMember(t *testing.T) {
	t.Parallel()

	t.Run("adds user to superadmin group", func(t *testing.T) {
		t.Parallel()

		bs, users, _, policies := newBootstrapFixture(t)
		ctx := t.Context()

		require.NoError(t, bs.BootstrapBasic(ctx, "superadmin@example.com", "pw"))

		newUser, err := users.GetByIdentity(ctx, string(domain.ProviderBasic), "superadmin@example.com")
		require.NoError(t, err)

		require.NoError(t, bs.EnsureMember(ctx, newUser.ID.String()))

		perms, err := policies.ListPermissionsForSubject(
			ctx,
			domain.GroupResource(domain.SystemGroupSuperAdmin),
		)
		require.NoError(t, err)
		assert.NotEmpty(t, perms)
	})

	t.Run("fails when superadmin group missing", func(t *testing.T) {
		t.Parallel()

		bs, _, _, _ := newBootstrapFixture(t)
		ctx := t.Context()

		err := bs.EnsureMember(ctx, uuid.NewString())
		require.ErrorContains(t, err, "ensure superadmin member")
	})
}
