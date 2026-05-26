package user_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
)

func TestService_ResetPassword(t *testing.T) {
	t.Parallel()

	t.Run("happy path: new hash stored, password_change_required=true", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)
		seedUser(t, st, targetEmail)

		err := st.svc.ResetPassword(t.Context(), adminActor(), targetEmail, resetPassword)
		require.NoError(t, err)

		persisted, err := st.users.Get(t.Context(), targetEmail)
		require.NoError(t, err)
		require.NotEmpty(t, persisted.PasswordHash)
		assert.NotEqual(t, resetPassword, persisted.PasswordHash) // hashed, not plaintext
		require.NoError(t, auth.VerifyPassword(persisted.PasswordHash, resetPassword))
		assert.True(t, persisted.PasswordChangeRequired)
	})

	t.Run("derived scope: actor with Group:Write on target's group can reset", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUser(t, st, targetEmail)
		seedGroup(t, st, "g1", "devs")
		addMemberships(t, st, []struct{ User, GroupName string }{
			{targetEmail, "devs"},
		})
		// No target permissions on the group → anti-escalation passes
		// trivially. Actor only needs Group:Write on g1.
		addPolicies(t, st, [][4]string{
			{actorEmail, domain.GroupResource("g1"), domain.ObjectGroup, domain.ActionWrite},
		})

		err := st.svc.ResetPassword(t.Context(), actor(), targetEmail, resetPassword)
		require.NoError(t, err)

		persisted, err := st.users.Get(t.Context(), targetEmail)
		require.NoError(t, err)
		require.NoError(t, auth.VerifyPassword(persisted.PasswordHash, resetPassword))
	})

	t.Run("forbidden when actor has no scope on target", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUser(t, st, targetEmail)

		err := st.svc.ResetPassword(t.Context(), actor(), targetEmail, resetPassword)
		require.ErrorIs(t, err, domain.ErrForbidden)

		// Password untouched.
		persisted, err := st.users.Get(t.Context(), targetEmail)
		require.NoError(t, err)
		assert.Empty(t, persisted.PasswordHash)
	})

	t.Run("anti-escalation: target has permissions the actor lacks", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUser(t, st, targetEmail)
		seedGroup(t, st, "g1", "elevated")
		addMemberships(t, st, []struct{ User, GroupName string }{
			{targetEmail, "elevated"},
		})

		// Target inherits config:write on ns-a from group:elevated. Actor
		// has only Group:Write on g1 (no config:write of its own).
		addPolicies(t, st, [][4]string{
			{casbin.GroupSubject("elevated"), "ns-a", domain.ObjectConfig, domain.ActionWrite},
			{actorEmail, domain.GroupResource("g1"), domain.ObjectGroup, domain.ActionWrite},
		})

		err := st.svc.ResetPassword(t.Context(), actor(), targetEmail, resetPassword)
		require.ErrorIs(t, err, domain.ErrPermissionEscalation)

		// Tx rolled back: password unchanged.
		persisted, err := st.users.Get(t.Context(), targetEmail)
		require.NoError(t, err)
		assert.Empty(t, persisted.PasswordHash)
	})

	t.Run("not found: missing user surfaces ErrNotFound", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		err := st.svc.ResetPassword(t.Context(), adminActor(), "ghost@example.com", resetPassword)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

// TestService_ResetPassword_SetPasswordErrorWrapped is fault injection: the
// in-tx SetPassword call returns a contrived error and we lock the
// wrapping message — refactor catches future regressions.
func TestService_ResetPassword_SetPasswordErrorWrapped(t *testing.T) {
	t.Parallel()

	m := setupServiceWithMockStore(t)
	seedAdminAllOnMockStack(t, m)

	m.store.EXPECT().
		Get(gomock.Any(), targetEmail).
		Return(&domain.User{Email: targetEmail}, nil)
	m.store.EXPECT().
		SetPassword(gomock.Any(), targetEmail, gomock.Any(), true).
		Return(errors.New("disk failure"))

	err := m.svc.ResetPassword(t.Context(), adminActor(), targetEmail, resetPassword)
	require.ErrorContains(t, err, "set password")
	require.ErrorContains(t, err, "disk failure")
}
