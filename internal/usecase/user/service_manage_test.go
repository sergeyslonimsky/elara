package user_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/user"
)

// ---- Create ------------------------------------------------------------------

func TestService_Create(t *testing.T) {
	t.Parallel()

	const newEmail = "new-user@example.com"

	t.Run(
		"basic-auth: stores user, password hash, and password_change_required=true",
		func(t *testing.T) {
			t.Parallel()

			st := setupServiceReal(t)
			seedAdminAll(t, st)

			res, err := st.svc.Create(t.Context(), adminActor(), user.CreateData{
				Email:           newEmail,
				DisplayName:     "New User",
				InitialPassword: "initial-password",
			})
			require.NoError(t, err)
			require.NotNil(t, res)

			// Persisted bbolt state matches the result and the input.
			persisted, err := st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), newEmail)
			require.NoError(t, err)
			assert.Equal(t, newEmail, persisted.Email)
			assert.Equal(t, "New User", persisted.DisplayName)
			// Password is stored as a bcrypt hash, not the plaintext.
			assert.NotEqual(t, "initial-password", persisted.PasswordHash)
			require.NoError(t, auth.VerifyPassword(persisted.PasswordHash, "initial-password"))
			// No initial groups → MembershipVersion initialised to 1 (proto3 omits 0).
			assert.Equal(t, int64(1), persisted.MembershipVersion)
			assert.Empty(t, res.GroupIDs)
			assert.Equal(t, int64(1), res.MembershipVersion)
		},
	)

	t.Run("OIDC pre-provision: empty password leaves Identities empty", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		res, err := st.svc.Create(t.Context(), adminActor(), user.CreateData{
			Email:       newEmail,
			DisplayName: "OIDC User",
		})
		require.NoError(t, err)

		// OIDC users have no identity until the first successful OIDC
		// callback links {oidc:<issuer>, sub} via email-fallback.
		persisted, err := st.users.GetByEmail(t.Context(), newEmail)
		require.NoError(t, err)
		assert.Empty(t, persisted.Identities)
		assert.Empty(t, persisted.PasswordHash)
		assert.False(t, persisted.PasswordChangeRequired)
		assert.Equal(t, persisted.ID, res.User.ID)
	})

	t.Run("with initial groups: persists memberships and bumps version to 1", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)
		seedGroup(t, st, "devs", "devs")
		seedGroup(t, st, "platform", "platform")

		res, err := st.svc.Create(t.Context(), adminActor(), user.CreateData{
			Email:           newEmail,
			DisplayName:     "New User",
			InitialPassword: "initial-password",
			InitialGroupIDs: []string{"devs", "platform"},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"devs", "platform"}, res.GroupIDs)
		assert.Equal(t, int64(1), res.MembershipVersion)

		// Casbin g-rules are stored under user.ID (UUID), not email.
		// Resolve the persisted user to get the minted UUID.
		persisted, err := st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), newEmail)
		require.NoError(t, err)
		assert.Equal(t, int64(1), persisted.MembershipVersion)

		roles, err := st.enforcer.GetRolesForUser(persisted.ID.String(), domain.MembershipDomain)
		require.NoError(t, err)
		assert.ElementsMatch(
			t,
			[]string{domain.GroupResource("devs"), domain.GroupResource("platform")},
			roles,
		)
	})

	t.Run("forbidden: no User:Create * and no initial groups", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		// Actor has nothing — proto requires User:Create * OR (groups +
		// Group:Write). Reject before any tx is opened.

		_, err := st.svc.Create(t.Context(), actor(), user.CreateData{
			Email:       newEmail,
			DisplayName: "New User",
		})
		require.ErrorIs(t, err, domain.ErrForbidden)

		// Nothing persisted: bbolt does not know about the user.
		_, err = st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), newEmail)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run(
		"forbidden: initial groups supplied but actor lacks Group:Write on one",
		func(t *testing.T) {
			t.Parallel()

			st := setupServiceReal(t)
			seedGroup(t, st, "devs", "devs")
			seedGroup(t, st, "platform", "platform")

			// Actor only has Group:Write on devs (policy subject is actorID).
			addPolicies(t, st, []policyRow{
				{actorID, domain.GroupResource("devs"), domain.ObjectGroup, domain.ActionWrite},
			})

			_, err := st.svc.Create(t.Context(), actor(), user.CreateData{
				Email:           newEmail,
				DisplayName:     "New User",
				InitialPassword: "p",
				InitialGroupIDs: []string{"devs", "platform"},
			})
			require.ErrorIs(t, err, domain.ErrForbidden)

			// No user persisted, no g-rules added.
			_, err = st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), newEmail)
			require.ErrorIs(t, err, domain.ErrNotFound)
		},
	)

	t.Run("anti-escalation: blocks add of group with permissions actor lacks", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedGroup(t, st, "elevated", "elevated")

		// elevated group grants config:write on ns-a; actor has only
		// Group:Write on elevated (no config:write of its own).
		// Policy subject is actorID (actor.UserID), not email.
		addPolicies(t, st, []policyRow{
			{domain.GroupResource("elevated"), "ns-a", domain.ObjectNamespace, domain.ActionWrite},
			{actorID, domain.GroupResource("elevated"), domain.ObjectGroup, domain.ActionWrite},
		})

		_, err := st.svc.Create(t.Context(), actor(), user.CreateData{
			Email:           newEmail,
			DisplayName:     "New User",
			InitialPassword: "p",
			InitialGroupIDs: []string{"elevated"},
		})
		require.ErrorIs(t, err, domain.ErrPermissionEscalation)

		// Tx rolled back: no user, no g-rules.
		_, err = st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), newEmail)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("validation error: invalid email", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		_, err := st.svc.Create(t.Context(), adminActor(), user.CreateData{
			Email:           "invalid-email",
			DisplayName:     "X",
			InitialPassword: "p",
		})
		require.ErrorContains(t, err, "normalize email")
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

	m.users.EXPECT().
		Create(gomock.Any(), gomock.AssignableToTypeOf(&domain.User{})).
		Return(errors.New("boom"))

	_, err := m.svc.Create(t.Context(), adminActor(), user.CreateData{
		Email:           "new-user@example.com",
		DisplayName:     "New User",
		InitialPassword: "initial-password",
	})
	require.ErrorContains(t, err, "create user")
	require.ErrorContains(t, err, "boom")
}

// ---- Delete ------------------------------------------------------------------

func TestService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("happy path: admin deletes target — bbolt row and g-rules gone", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)
		seedUserWithID(t, st, targetID, targetEmail)
		seedGroup(t, st, "g1", "devs")
		// Memberships are stored under user.ID (UUID) in Casbin.
		addMemberships(t, st, []struct{ User, GroupName string }{
			{targetID, "devs"},
		})

		err := st.svc.Delete(t.Context(), adminActor(), targetUUID)
		require.NoError(t, err)

		_, err = st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
		require.ErrorIs(t, err, domain.ErrNotFound)
		roles, err := st.enforcer.GetRolesForUser(targetID, domain.MembershipDomain)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("rejects self-delete", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		// Seed the admin user with the stable adminID so that actor.UserID == user.ID.
		seedUserWithID(t, st, adminID, adminEmail)

		err := st.svc.Delete(t.Context(), adminActor(), adminUUID)
		require.ErrorContains(t, err, "cannot delete your own account")
		require.True(t, domain.IsValidationError(err))
	})

	t.Run("not found: missing user returns wrapped ErrNotFound", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		err := st.svc.Delete(t.Context(), adminActor(), ghostUUID)
		require.ErrorIs(t, err, domain.ErrNotFound)
		require.ErrorContains(t, err, "get user")
	})

	t.Run("forbidden when actor has no scope over target", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUserWithID(t, st, targetID, targetEmail)

		err := st.svc.Delete(t.Context(), actor(), targetUUID)
		require.ErrorIs(t, err, domain.ErrForbidden)

		// Target still exists.
		_, err = st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
		require.NoError(t, err)
	})

	t.Run("anti-escalation: actor lacks one of target's permissions", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUserWithID(t, st, targetID, targetEmail)
		seedGroup(t, st, "g1", "devs")
		// Memberships stored under user.ID (UUID), not email.
		addMemberships(t, st, []struct{ User, GroupName string }{
			{targetID, "devs"},
		})

		// Actor: Group:Write on devs (can write the target) but NO config:write
		// permission that the target's group grants. Policy subject is actorID.
		addPolicies(t, st, []policyRow{
			{domain.GroupResource("devs"), "ns-a", domain.ObjectNamespace, domain.ActionWrite},
			{actorID, domain.GroupResource("devs"), domain.ObjectGroup, domain.ActionWrite},
		})

		err := st.svc.Delete(t.Context(), actor(), targetUUID)
		require.ErrorIs(t, err, domain.ErrPermissionEscalation)

		_, err = st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
		require.NoError(t, err)
	})

	t.Run(
		"last-admin guard: refuses to delete sole member of superadmin group",
		func(t *testing.T) {
			t.Parallel()

			st := setupServiceReal(t)
			// The caller reaches the guard via a direct wildcard User:Write policy
			// (not group membership), so it is not itself a superadmin-group member.
			// Policy subject is adminID (actor.UserID).
			seedUserWithID(t, st, targetID, targetEmail)
			addPolicies(t, st, []policyRow{
				{adminID, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
			})
			// Target is the sole member of the superadmin group — membership is
			// stored under targetID (user.ID) in Casbin.
			addMemberships(t, st, []struct{ User, GroupName string }{
				{targetID, domain.SystemGroupSuperAdmin},
			})

			err := st.svc.Delete(t.Context(), adminActor(), targetUUID)
			require.ErrorContains(t, err, "cannot delete the last admin")
			require.True(t, domain.IsValidationError(err))

			// Target still exists.
			_, err = st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
			require.NoError(t, err)
		},
	)
}

// ---- Get ---------------------------------------------------------------------

func TestService_Get(t *testing.T) {
	t.Parallel()

	t.Run(
		"happy path with User:Read *: returns user and no visible groups when target has none",
		func(t *testing.T) {
			t.Parallel()

			st := setupServiceReal(t)
			seedAdminAll(t, st)
			seedUserWithID(t, st, targetID, targetEmail)

			got, err := st.svc.Get(t.Context(), adminActor(), targetUUID)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, targetEmail, got.User.Email)
			assert.Empty(t, got.VisibleGroupIDs)
			assert.Equal(t, int64(0), got.MembershipVersion)
		},
	)

	t.Run(
		"out-of-scope returns ErrNotFound (not Forbidden) for enumeration safety",
		func(t *testing.T) {
			t.Parallel()

			st := setupServiceReal(t)
			seedUserWithID(t, st, targetID, targetEmail)

			_, err := st.svc.Get(t.Context(), actor(), targetUUID)
			require.ErrorIs(t, err, domain.ErrNotFound)
		},
	)

	t.Run("missing user returns ErrNotFound", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		_, err := st.svc.Get(t.Context(), adminActor(), ghostUUID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("VisibleGroupIDs is filtered through Group:Read", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedUserWithID(t, st, targetID, targetEmail)
		seedGroup(t, st, "visible", "visible")
		seedGroup(t, st, "hidden", "hidden")
		// Memberships stored under targetID (user.ID) in Casbin.
		addMemberships(t, st, []struct{ User, GroupName string }{
			{targetID, "visible"},
			{targetID, "hidden"},
		})

		// Actor can read group:visible only (also Group:Read used to grant
		// scope over the user itself). Policy subject is actorID.
		addPolicies(t, st, []policyRow{
			{actorID, domain.GroupResource("visible"), domain.ObjectGroup, domain.ActionRead},
		})

		got, err := st.svc.Get(t.Context(), actor(), targetUUID)
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
		GetByID(gomock.Any(), targetUUID).
		Return(nil, errors.New("disk failure"))

	_, err := m.svc.Get(t.Context(), adminActor(), targetUUID)
	require.ErrorContains(t, err, "get user")
	require.ErrorContains(t, err, "disk failure")
}
