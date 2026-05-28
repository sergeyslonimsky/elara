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
	"github.com/sergeyslonimsky/elara/internal/usecase/user"
)

// ---- Create ------------------------------------------------------------------

func TestService_Create(t *testing.T) {
	t.Parallel()

	const newEmail = "new-user@example.com"

	t.Run("basic-auth: stores user, password hash, and password_change_required=true", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		res, err := st.svc.Create(t.Context(), adminActor(), user.CreateData{
			Email:           newEmail,
			Name:            "New User",
			InitialPassword: "initial-password",
		})
		require.NoError(t, err)
		require.NotNil(t, res)

		// Persisted bbolt state matches the result and the input.
		persisted, err := st.users.Get(t.Context(), newEmail)
		require.NoError(t, err)
		assert.Equal(t, newEmail, persisted.Email)
		assert.Equal(t, "New User", persisted.Name)
		assert.Equal(t, domain.ProviderBasicAuth, persisted.Provider)
		assert.True(t, persisted.PasswordChangeRequired)
		// Password is stored as a bcrypt hash, not the plaintext.
		assert.NotEqual(t, "initial-password", persisted.PasswordHash)
		require.NoError(t, auth.VerifyPassword(persisted.PasswordHash, "initial-password"))
		// No initial groups → MembershipVersion initialised to 1 (proto3 omits 0).
		assert.Equal(t, int64(1), persisted.MembershipVersion)
		assert.Empty(t, res.GroupIDs)
		assert.Equal(t, int64(1), res.MembershipVersion)
	})

	t.Run("OIDC: empty password sets ProviderOIDC and no password hash", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		res, err := st.svc.Create(t.Context(), adminActor(), user.CreateData{
			Email: newEmail,
			Name:  "OIDC User",
		})
		require.NoError(t, err)
		assert.Equal(t, domain.ProviderOIDC, res.User.Provider)

		persisted, err := st.users.Get(t.Context(), newEmail)
		require.NoError(t, err)
		assert.Equal(t, domain.ProviderOIDC, persisted.Provider)
		assert.Empty(t, persisted.PasswordHash)
		assert.False(t, persisted.PasswordChangeRequired)
	})

	t.Run("with initial groups: persists memberships and bumps version to 1", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)
		seedGroup(t, st, "g1", "devs")
		seedGroup(t, st, "g2", "platform")

		res, err := st.svc.Create(t.Context(), adminActor(), user.CreateData{
			Email:           newEmail,
			Name:            "New User",
			InitialPassword: "initial-password",
			InitialGroupIDs: []string{"g1", "g2"},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"g1", "g2"}, res.GroupIDs)
		assert.Equal(t, int64(1), res.MembershipVersion)

		// Casbin g-rules reflect the membership additions.
		roles, err := st.enforcer.GetRolesForUser(newEmail, domain.MembershipDomain)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{casbin.GroupSubject("devs"), casbin.GroupSubject("platform")}, roles)

		// bbolt MembershipVersion mirrors the result.
		persisted, err := st.users.Get(t.Context(), newEmail)
		require.NoError(t, err)
		assert.Equal(t, int64(1), persisted.MembershipVersion)
	})

	t.Run("forbidden: no User:Create * and no initial groups", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		// Actor has nothing — proto requires User:Create * OR (groups +
		// Group:Write). Reject before any tx is opened.

		_, err := st.svc.Create(t.Context(), actor(), user.CreateData{
			Email: newEmail,
			Name:  "New User",
		})
		require.ErrorIs(t, err, domain.ErrForbidden)

		// Nothing persisted: bbolt does not know about the user.
		_, err = st.users.Get(t.Context(), newEmail)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("forbidden: initial groups supplied but actor lacks Group:Write on one", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedGroup(t, st, "g1", "devs")
		seedGroup(t, st, "g2", "platform")

		// Actor only has Group:Write on g1.
		addPolicies(t, st, []policyRow{
			{actorEmail, domain.GroupResource("g1"), domain.ObjectGroup, domain.ActionWrite},
		})

		_, err := st.svc.Create(t.Context(), actor(), user.CreateData{
			Email:           newEmail,
			Name:            "New User",
			InitialPassword: "p",
			InitialGroupIDs: []string{"g1", "g2"},
		})
		require.ErrorIs(t, err, domain.ErrForbidden)

		// No user persisted, no g-rules added.
		_, err = st.users.Get(t.Context(), newEmail)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("anti-escalation: blocks add of group with permissions actor lacks", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedGroup(t, st, "g1", "elevated")

		// elevated group grants config:write on ns-a; actor has only
		// Group:Write on g1 (no config:write of its own).
		addPolicies(t, st, []policyRow{
			{casbin.GroupSubject("elevated"), "ns-a", domain.ObjectConfig, domain.ActionWrite},
			{actorEmail, domain.GroupResource("g1"), domain.ObjectGroup, domain.ActionWrite},
		})

		_, err := st.svc.Create(t.Context(), actor(), user.CreateData{
			Email:           newEmail,
			Name:            "New User",
			InitialPassword: "p",
			InitialGroupIDs: []string{"g1"},
		})
		require.ErrorIs(t, err, domain.ErrPermissionEscalation)

		// Tx rolled back: no user, no g-rules.
		_, err = st.users.Get(t.Context(), newEmail)
		require.ErrorIs(t, err, domain.ErrNotFound)
		roles, err := st.enforcer.GetRolesForUser(newEmail, domain.MembershipDomain)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("validation error: invalid email", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		_, err := st.svc.Create(t.Context(), adminActor(), user.CreateData{
			Email:           "invalid-email",
			Name:            "X",
			InitialPassword: "p",
		})
		require.ErrorContains(t, err, "validate user")
		require.True(t, domain.IsValidationError(err))
	})
}

// TestService_Create_UpsertErrorWrapped exercises the error-wrapping contract
// when the underlying store fails during the in-tx Upsert. Mock-based, fault
// injection only.
func TestService_Create_UpsertErrorWrapped(t *testing.T) {
	t.Parallel()

	m := setupServiceWithMockStore(t)
	seedAdminAllOnMockStack(t, m)

	m.store.EXPECT().
		Upsert(gomock.Any(), gomock.AssignableToTypeOf(&domain.User{})).
		Return(errors.New("boom"))

	_, err := m.svc.Create(t.Context(), adminActor(), user.CreateData{
		Email:           "new-user@example.com",
		Name:            "New User",
		InitialPassword: "initial-password",
	})
	require.ErrorContains(t, err, "upsert user")
	require.ErrorContains(t, err, "boom")
}

// ---- Delete ------------------------------------------------------------------

func TestService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("happy path: admin deletes target — bbolt row and g-rules gone", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)
		seedUser(t, st, targetEmail)
		seedGroup(t, st, "g1", "devs")
		addMemberships(t, st, []struct{ User, GroupName string }{
			{targetEmail, "devs"},
		})

		err := st.svc.Delete(t.Context(), adminActor(), targetEmail)
		require.NoError(t, err)

		_, err = st.users.Get(t.Context(), targetEmail)
		require.ErrorIs(t, err, domain.ErrNotFound)
		roles, err := st.enforcer.GetRolesForUser(targetEmail, domain.MembershipDomain)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("rejects self-delete", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)

		err := st.svc.Delete(t.Context(), adminActor(), adminEmail)
		require.ErrorContains(t, err, "cannot delete your own account")
		require.True(t, domain.IsValidationError(err))
	})

	t.Run("not found: missing user returns wrapped ErrNotFound", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		err := st.svc.Delete(t.Context(), adminActor(), "ghost@example.com")
		require.ErrorIs(t, err, domain.ErrNotFound)
		require.ErrorContains(t, err, "get user")
	})

	t.Run("forbidden when actor has no scope over target", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUser(t, st, targetEmail)

		err := st.svc.Delete(t.Context(), actor(), targetEmail)
		require.ErrorIs(t, err, domain.ErrForbidden)

		// Target still exists.
		_, err = st.users.Get(t.Context(), targetEmail)
		require.NoError(t, err)
	})

	t.Run("anti-escalation: actor lacks one of target's permissions", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUser(t, st, targetEmail)
		seedGroup(t, st, "g1", "devs")
		addMemberships(t, st, []struct{ User, GroupName string }{
			{targetEmail, "devs"},
		})

		// Actor: Group:Write on g1 (can write the target) but NO config:write
		// permission that the target's group grants.
		addPolicies(t, st, []policyRow{
			{casbin.GroupSubject("devs"), "ns-a", domain.ObjectConfig, domain.ActionWrite},
			{actorEmail, domain.GroupResource("g1"), domain.ObjectGroup, domain.ActionWrite},
		})

		err := st.svc.Delete(t.Context(), actor(), targetEmail)
		require.ErrorIs(t, err, domain.ErrPermissionEscalation)

		_, err = st.users.Get(t.Context(), targetEmail)
		require.NoError(t, err)
	})

	t.Run("last-admin guard: refuses to delete sole holder of admin role", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		// Sole admin: a separate actor (so self-delete check doesn't shadow
		// the last-admin guard) is the unique RoleAdmin/* holder. The caller
		// holds the wildcard User:Write needed to reach the guard.
		seedUser(t, st, targetEmail)
		addPolicies(t, st, []policyRow{
			{adminEmail, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
		})
		// Assign target the admin role on the wildcard domain so they are
		// the sole holder of the admin grant.
		addRoleForUser(t, st, targetEmail, string(domain.RoleAdmin), domain.DomainAll)

		err := st.svc.Delete(t.Context(), adminActor(), targetEmail)
		require.ErrorContains(t, err, "cannot delete the last admin")
		require.True(t, domain.IsValidationError(err))

		// Target still exists.
		_, err = st.users.Get(t.Context(), targetEmail)
		require.NoError(t, err)
	})
}

// ---- Get ---------------------------------------------------------------------

func TestService_Get(t *testing.T) {
	t.Parallel()

	t.Run("happy path with User:Read *: returns user and no visible groups when target has none", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)
		seedUser(t, st, targetEmail)

		got, err := st.svc.Get(t.Context(), adminActor(), targetEmail)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, targetEmail, got.User.Email)
		assert.Empty(t, got.VisibleGroupIDs)
		assert.Equal(t, int64(0), got.MembershipVersion)
	})

	t.Run("out-of-scope returns ErrNotFound (not Forbidden) for enumeration safety", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUser(t, st, targetEmail)

		_, err := st.svc.Get(t.Context(), actor(), targetEmail)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("missing user returns ErrNotFound", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		_, err := st.svc.Get(t.Context(), adminActor(), "ghost@example.com")
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("VisibleGroupIDs is filtered through Group:Read", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUser(t, st, targetEmail)
		seedGroup(t, st, "visible", "visible-grp")
		seedGroup(t, st, "hidden", "hidden-grp")
		addMemberships(t, st, []struct{ User, GroupName string }{
			{targetEmail, "visible-grp"},
			{targetEmail, "hidden-grp"},
		})

		// Actor can read group:visible only (also Group:Read used to grant
		// scope over the user itself).
		addPolicies(t, st, []policyRow{
			{actorEmail, domain.GroupResource("visible"), domain.ObjectGroup, domain.ActionRead},
		})

		got, err := st.svc.Get(t.Context(), actor(), targetEmail)
		require.NoError(t, err)
		assert.Equal(t, []string{"visible"}, got.VisibleGroupIDs)
	})
}

// TestService_Get_StoreErrorWrapped is the lone mock-based test for Get —
// pinning the error-wrapping message when the store returns an arbitrary
// failure.
func TestService_Get_StoreErrorWrapped(t *testing.T) {
	t.Parallel()

	m := setupServiceWithMockStore(t)
	seedAdminAllOnMockStack(t, m)

	m.store.EXPECT().
		Get(gomock.Any(), targetEmail).
		Return(nil, errors.New("disk failure"))

	_, err := m.svc.Get(t.Context(), adminActor(), targetEmail)
	require.ErrorContains(t, err, "get user")
	require.ErrorContains(t, err, "disk failure")
}
