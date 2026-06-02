package user_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestService_Reactivate(t *testing.T) {
	t.Parallel()

	t.Run("success: deactivated user becomes active and DB reflects change", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		targetUUID := uuid.New()
		err := st.users.Create(t.Context(), &domain.User{
			ID:          targetUUID,
			Email:       targetEmail,
			DisplayName: targetEmail,
			Status:      domain.UserStatusDeactivated,
			Identities: []domain.Identity{
				{Provider: domain.ProviderBasic, Subject: targetEmail},
			},
		})
		require.NoError(t, err)

		result, err := st.svc.Reactivate(t.Context(), adminActor(), targetUUID)
		require.NoError(t, err)
		assert.Equal(t, domain.UserStatusActive, result.User.Status)

		// Verify user is updated in database.
		dbUser, err := st.users.GetByID(t.Context(), targetUUID)
		require.NoError(t, err)
		assert.Equal(t, domain.UserStatusActive, dbUser.Status)
	})

	t.Run("cannot reactivate your own account", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		// Seed the admin as deactivated so the self-guard fires before the
		// status check — the self-target validation is independent of status.
		err := st.users.Create(t.Context(), &domain.User{
			ID:          uuid.MustParse(adminID),
			Email:       adminEmail,
			DisplayName: adminEmail,
			Status:      domain.UserStatusDeactivated,
			Identities: []domain.Identity{
				{Provider: domain.ProviderBasic, Subject: adminEmail},
			},
		})
		require.NoError(t, err)

		_, err = st.svc.Reactivate(t.Context(), adminActor(), adminUUID)
		require.Error(t, err)
		assert.True(t, domain.IsValidationError(err))
		require.ErrorContains(t, err, "cannot reactivate your own account")
	})

	t.Run("already active returns validation error", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		targetUUID := uuid.New()
		err := st.users.Create(t.Context(), &domain.User{
			ID:          targetUUID,
			Email:       targetEmail,
			DisplayName: targetEmail,
			Status:      domain.UserStatusActive,
			Identities: []domain.Identity{
				{Provider: domain.ProviderBasic, Subject: targetEmail},
			},
		})
		require.NoError(t, err)

		_, err = st.svc.Reactivate(t.Context(), adminActor(), targetUUID)
		require.Error(t, err)
		assert.True(t, domain.IsValidationError(err))
		require.ErrorContains(t, err, "user is already active")
	})

	t.Run("system user is rejected with ErrSystemImmutable", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		targetUUID := uuid.New()
		err := st.users.Create(t.Context(), &domain.User{
			ID:          targetUUID,
			Email:       targetEmail,
			DisplayName: targetEmail,
			Status:      domain.UserStatusDeactivated,
			System:      true,
			Identities: []domain.Identity{
				{Provider: domain.ProviderBasic, Subject: targetEmail},
			},
		})
		require.NoError(t, err)

		_, err = st.svc.Reactivate(t.Context(), adminActor(), targetUUID)
		require.ErrorIs(t, err, domain.ErrSystemImmutable)
	})

	t.Run("unauthorized actor without User:Write is rejected", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		// actorID has no grants — authorization must fail before any mutation.
		seedUserWithID(t, st, actorID, actorEmail)

		targetUUID := uuid.New()
		err := st.users.Create(t.Context(), &domain.User{
			ID:          targetUUID,
			Email:       targetEmail,
			DisplayName: targetEmail,
			Status:      domain.UserStatusDeactivated,
			Identities: []domain.Identity{
				{Provider: domain.ProviderBasic, Subject: targetEmail},
			},
		})
		require.NoError(t, err)

		_, err = st.svc.Reactivate(t.Context(), actor(), targetUUID)
		require.ErrorIs(t, err, domain.ErrForbidden)

		// Verify user status is unchanged in database.
		dbUser, err := st.users.GetByID(t.Context(), targetUUID)
		require.NoError(t, err)
		assert.Equal(t, domain.UserStatusDeactivated, dbUser.Status)
	})
}
