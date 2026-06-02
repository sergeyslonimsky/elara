package user_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func TestService_ResetPassword(t *testing.T) {
	t.Parallel()

	t.Run("happy path: new hash stored, password_change_required=true", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)
		seedUserWithID(t, st, targetID, targetEmail)

		err := st.svc.ResetPassword(t.Context(), adminActor(), targetUUID, resetPassword)
		require.NoError(t, err)

		persisted, err := st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
		require.NoError(t, err)
		require.NotEmpty(t, persisted.PasswordHash)
		assert.NotEqual(t, resetPassword, persisted.PasswordHash) // hashed, not plaintext
		require.NoError(t, auth.VerifyPassword(persisted.PasswordHash, resetPassword))
		assert.True(t, persisted.PasswordChangeRequired)
	})

	t.Run("derived scope: actor with Group:Write on target's group can reset", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUserWithID(t, st, targetID, targetEmail)
		seedGroup(t, st, "devs", "devs")
		// Membership stored under targetID (user.ID) in Casbin.
		addMemberships(t, st, []struct{ User, GroupName string }{
			{targetID, "devs"},
		})
		// No target permissions on the group → anti-escalation passes
		// trivially. Actor only needs Group:Write on devs. Policy subject is actorID.
		addPolicies(t, st, []policyRow{
			{actorID, domain.GroupResource("devs"), domain.ObjectGroup, domain.ActionWrite},
		})

		err := st.svc.ResetPassword(t.Context(), actor(), targetUUID, resetPassword)
		require.NoError(t, err)

		persisted, err := st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
		require.NoError(t, err)
		require.NoError(t, auth.VerifyPassword(persisted.PasswordHash, resetPassword))
	})

	t.Run("forbidden when actor has no scope on target", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUserWithID(t, st, targetID, targetEmail)

		err := st.svc.ResetPassword(t.Context(), actor(), targetUUID, resetPassword)
		require.ErrorIs(t, err, domain.ErrForbidden)

		// Password untouched.
		persisted, err := st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
		require.NoError(t, err)
		assert.Empty(t, persisted.PasswordHash)
	})

	t.Run("anti-escalation: target has permissions the actor lacks", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUserWithID(t, st, targetID, targetEmail)
		seedGroup(t, st, "g1", "elevated")
		// Membership stored under targetID (user.ID) in Casbin.
		addMemberships(t, st, []struct{ User, GroupName string }{
			{targetID, "elevated"},
		})

		// Target inherits config:write on ns-a from group:elevated. Actor
		// has only Group:Write on elevated (no config:write of its own). Policy subject is actorID.
		addPolicies(t, st, []policyRow{
			{domain.GroupResource("elevated"), "ns-a", domain.ObjectNamespace, domain.ActionWrite},
			{actorID, domain.GroupResource("elevated"), domain.ObjectGroup, domain.ActionWrite},
		})

		err := st.svc.ResetPassword(t.Context(), actor(), targetUUID, resetPassword)
		require.ErrorIs(t, err, domain.ErrPermissionEscalation)

		// Tx rolled back: password unchanged.
		persisted, err := st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
		require.NoError(t, err)
		assert.Empty(t, persisted.PasswordHash)
	})

	t.Run("not found: missing user surfaces ErrNotFound", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		err := st.svc.ResetPassword(t.Context(), adminActor(), ghostUUID, resetPassword)
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
		GetByID(gomock.Any(), targetUUID).
		Return(&domain.User{ID: targetUUID, Email: targetEmail}, nil)
	m.store.EXPECT().
		SetPassword(gomock.Any(), targetUUID, gomock.Any(), true).
		Return(errors.New("disk failure"))

	err := m.svc.ResetPassword(t.Context(), adminActor(), targetUUID, resetPassword)
	require.ErrorContains(t, err, "set password")
	require.ErrorContains(t, err, "disk failure")
}
