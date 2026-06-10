package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// TestBootstrapBasic_SingleBasicIdentity covers the EL-50 §3.3.2 contract:
// the bootstrap admin carries exactly one identity ({basic, <username>}) and
// is identifiable via the System: true flag. The legacy "double identity"
// pattern ({system, bootstrap} + {basic, ...}) is gone.
func TestBootstrapBasic_SingleBasicIdentity(t *testing.T) {
	t.Parallel()

	t.Run("first run creates a system user with a single basic identity", func(t *testing.T) {
		t.Parallel()

		bs, users, _, _ := newBootstrapFixture(t)
		ctx := t.Context()

		require.NoError(t, bs.BootstrapBasic(ctx, "admin@example.com", "pw"))

		user, err := users.GetSystemUser(ctx)
		require.NoError(t, err)
		require.True(t, user.System)
		require.Len(t, user.Identities, 1)
		assert.Equal(t, domain.ProviderBasic, user.Identities[0].Provider)
		assert.Equal(t, "admin@example.com", user.Identities[0].Subject)
		assert.Equal(t, "admin@example.com", user.Email)
	})

	t.Run("repeat run is idempotent — no duplicate users or identities", func(t *testing.T) {
		t.Parallel()

		bs, users, _, _ := newBootstrapFixture(t)
		ctx := t.Context()

		require.NoError(t, bs.BootstrapBasic(ctx, "admin@example.com", "pw"))

		userAfterFirst, err := users.GetSystemUser(ctx)
		require.NoError(t, err)
		firstID := userAfterFirst.ID

		require.NoError(t, bs.BootstrapBasic(ctx, "admin@example.com", "pw"))

		userAfterSecond, err := users.GetSystemUser(ctx)
		require.NoError(t, err)
		assert.Equal(t, firstID, userAfterSecond.ID, "UUID must be stable across bootstrap runs")
		assert.Len(t, userAfterSecond.Identities, 1, "no duplicate identities must be created")

		allUsers, err := users.ListAll(ctx)
		require.NoError(t, err)
		assert.Len(t, allUsers, 1, "exactly one user must exist after two bootstrap runs")
	})

	t.Run("username rename swaps the basic identity in place", func(t *testing.T) {
		t.Parallel()

		bs, users, _, _ := newBootstrapFixture(t)
		ctx := t.Context()

		require.NoError(t, bs.BootstrapBasic(ctx, "admin1@example.com", "pw"))

		userBefore, err := users.GetSystemUser(ctx)
		require.NoError(t, err)
		originalID := userBefore.ID

		require.NoError(t, bs.BootstrapBasic(ctx, "admin2@example.com", "pw"))

		userAfter, err := users.GetSystemUser(ctx)
		require.NoError(t, err)
		assert.Equal(t, originalID, userAfter.ID, "UUID must not change after username rename")
		require.Len(t, userAfter.Identities, 1, "still exactly one identity")
		assert.Equal(t, "admin2@example.com", userAfter.Identities[0].Subject)
		assert.Equal(t, "admin2@example.com", userAfter.Email, "Email tracks the new username")

		// Old subject is no longer resolvable.
		_, err = users.GetByIdentity(ctx, string(domain.ProviderBasic), "admin1@example.com")
		require.ErrorIs(t, err, storage.ErrResourceNotFound)

		// New subject resolves.
		userByNewName, err := users.GetByIdentity(ctx, string(domain.ProviderBasic), "admin2@example.com")
		require.NoError(t, err)
		assert.Equal(t, originalID, userByNewName.ID)

		allUsers, err := users.ListAll(ctx)
		require.NoError(t, err)
		assert.Len(t, allUsers, 1, "rename must not create a second user")
	})
}
