package user_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/user"
)

const (
	writableID = "writable"
	escalateID = "escalate"
)

// updateGroupsSetup builds the canonical pre-state for the UpdateGroups
// tests: three seeded users (admin, actor, target) and two seeded groups
// ("writable" and "escalate"). Per-case grants are added via addPolicies.
//
// Users are seeded with stable IDs that match their AuthInfo.UserID so that
// Casbin membership g-rules (stored under user.ID) agree with policy lookups.
func updateGroupsSetup(t *testing.T) realStack {
	t.Helper()

	st := setupServiceReal(t)
	seedUserWithID(t, st, targetID, targetEmail)
	seedUserWithID(t, st, actorID, actorEmail)
	seedUserWithID(t, st, adminID, adminEmail)
	seedGroup(t, st, writableID, "writable")
	seedGroup(t, st, escalateID, "escalate")

	return st
}

// TestService_UpdateGroups_Apply exercises the happy / authorization paths
// of UpdateGroups end-to-end against the real bbolt + Casbin stack. We
// assert on observable state (Casbin g-rules, persisted MembershipVersion,
// returned VisibleGroupIDs) rather than mock-interaction counts.
func TestService_UpdateGroups_Apply(t *testing.T) {
	t.Parallel()

	t.Run("happy path: admin adds two empty groups, version bumps to 1", func(t *testing.T) {
		t.Parallel()

		st := updateGroupsSetup(t)
		addPolicies(t, st, []policyRow{
			{adminID, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
		})

		res, err := st.svc.UpdateGroups(t.Context(), adminActor(), user.UpdateGroupsData{
			UserID:      targetUUID,
			AddGroupIDs: []string{writableID, escalateID},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.MembershipVersion)
		assert.ElementsMatch(t, []string{writableID, escalateID}, res.VisibleGroupIDs)

		roles, err := st.enforcer.GetRolesForUser(targetID, domain.MembershipDomain)
		require.NoError(t, err)
		assert.ElementsMatch(t,
			[]string{domain.GroupResource("writable"), domain.GroupResource("escalate")},
			roles,
		)
		// Persisted MembershipVersion mirrors the result.
		persisted, err := st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
		require.NoError(t, err)
		assert.Equal(t, int64(1), persisted.MembershipVersion)
	})

	t.Run("forbidden when actor lacks Group:Write on any add id", func(t *testing.T) {
		t.Parallel()

		st := updateGroupsSetup(t)
		// Actor can write to writable only; request touches escalate too.
		addPolicies(t, st, []policyRow{
			{actorID, domain.GroupResource(writableID), domain.ObjectGroup, domain.ActionWrite},
		})

		_, err := st.svc.UpdateGroups(t.Context(), actor(), user.UpdateGroupsData{
			UserID:      targetUUID,
			AddGroupIDs: []string{writableID, escalateID},
		})
		require.ErrorIs(t, err, domain.ErrForbidden)

		// Transaction rolled back — no memberships persisted.
		roles, err := st.enforcer.GetRolesForUser(targetID, domain.MembershipDomain)
		require.NoError(t, err)
		assert.Empty(t, roles)
		persisted, err := st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
		require.NoError(t, err)
		assert.Equal(t, int64(0), persisted.MembershipVersion)
	})

	t.Run("escalation blocked when actor lacks one group permission", func(t *testing.T) {
		t.Parallel()

		st := updateGroupsSetup(t)
		addPolicies(t, st, []policyRow{
			{domain.GroupResource("escalate"), "ns-a", domain.ObjectNamespace, domain.ActionWrite},
			{actorID, domain.GroupResource(escalateID), domain.ObjectGroup, domain.ActionWrite},
		})

		_, err := st.svc.UpdateGroups(t.Context(), actor(), user.UpdateGroupsData{
			UserID:      targetUUID,
			AddGroupIDs: []string{escalateID},
		})
		require.ErrorIs(t, err, domain.ErrPermissionEscalation)

		roles, err := st.enforcer.GetRolesForUser(targetID, domain.MembershipDomain)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("remove path: only Group:Write needed (no anti-escalation re-check)", func(t *testing.T) {
		t.Parallel()

		st := updateGroupsSetup(t)
		addPolicies(t, st, []policyRow{
			{adminID, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
			{domain.GroupResource("escalate"), "ns-a", domain.ObjectNamespace, domain.ActionWrite},
			{actorID, domain.GroupResource(escalateID), domain.ObjectGroup, domain.ActionWrite},
		})

		// Admin first adds target to escalate.
		_, err := st.svc.UpdateGroups(t.Context(), adminActor(), user.UpdateGroupsData{
			UserID:      targetUUID,
			AddGroupIDs: []string{escalateID},
		})
		require.NoError(t, err)

		// Actor removes; only Group:Write needed, no config-write perm required.
		res, err := st.svc.UpdateGroups(t.Context(), actor(), user.UpdateGroupsData{
			UserID:         targetUUID,
			RemoveGroupIDs: []string{escalateID},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(2), res.MembershipVersion)

		roles, err := st.enforcer.GetRolesForUser(targetID, domain.MembershipDomain)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run(
		"forbidden on remove of group caller lacks Group:Write — proto contract (oracle safety)",
		func(t *testing.T) {
			t.Parallel()

			// authorizeGroupDeltas runs on the REQUESTED union, not the
			// effective delta. Submitting "remove X" without Group:Write on X
			// must be Forbidden even when target isn't a member, otherwise the
			// no-op-success path would leak membership info.
			st := updateGroupsSetup(t)

			_, err := st.svc.UpdateGroups(t.Context(), actor(), user.UpdateGroupsData{
				UserID:         targetUUID,
				RemoveGroupIDs: []string{writableID},
			})
			require.ErrorIs(t, err, domain.ErrForbidden)
		},
	)

	t.Run(
		"idempotent: re-adding existing membership is a no-op, version unchanged",
		func(t *testing.T) {
			t.Parallel()

			st := updateGroupsSetup(t)
			addPolicies(t, st, []policyRow{
				{adminID, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
			})

			// First add: version 0 → 1.
			res1, err := st.svc.UpdateGroups(t.Context(), adminActor(), user.UpdateGroupsData{
				UserID:      targetUUID,
				AddGroupIDs: []string{writableID},
			})
			require.NoError(t, err)
			require.Equal(t, int64(1), res1.MembershipVersion)

			// Second add of same id: effective add is empty, version stays at 1.
			res2, err := st.svc.UpdateGroups(t.Context(), adminActor(), user.UpdateGroupsData{
				UserID:      targetUUID,
				AddGroupIDs: []string{writableID},
			})
			require.NoError(t, err)
			assert.Equal(t, int64(1), res2.MembershipVersion)

			// Persisted version mirrors the no-op contract.
			persisted, err := st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
			require.NoError(t, err)
			assert.Equal(t, int64(1), persisted.MembershipVersion)

			roles, err := st.enforcer.GetRolesForUser(targetID, domain.MembershipDomain)
			require.NoError(t, err)
			assert.Equal(t, []string{domain.GroupResource("writable")}, roles)
		},
	)

	t.Run("rejects id in both add and remove (validation)", func(t *testing.T) {
		t.Parallel()

		st := updateGroupsSetup(t)
		addPolicies(t, st, []policyRow{
			{adminID, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
		})

		_, err := st.svc.UpdateGroups(t.Context(), adminActor(), user.UpdateGroupsData{
			UserID:         targetUUID,
			AddGroupIDs:    []string{writableID},
			RemoveGroupIDs: []string{writableID},
		})
		require.ErrorContains(t, err, "appears in both add and remove")
		require.True(t, domain.IsValidationError(err))
	})
}

// TestService_UpdateGroups_Version covers the optimistic-lock surface.
func TestService_UpdateGroups_Version(t *testing.T) {
	t.Parallel()

	t.Run("mismatch returns ErrVersionConflict", func(t *testing.T) {
		t.Parallel()

		st := updateGroupsSetup(t)
		addPolicies(t, st, []policyRow{
			{adminID, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
		})

		// Current MembershipVersion is 0; caller submits 5.
		_, err := st.svc.UpdateGroups(t.Context(), adminActor(), user.UpdateGroupsData{
			UserID:                    targetUUID,
			AddGroupIDs:               []string{writableID},
			ExpectedMembershipVersion: new(int64(5)),
		})
		require.ErrorIs(t, err, domain.ErrVersionConflict)

		roles, err := st.enforcer.GetRolesForUser(targetID, domain.MembershipDomain)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("matches: applies and bumps version", func(t *testing.T) {
		t.Parallel()

		st := updateGroupsSetup(t)
		addPolicies(t, st, []policyRow{
			{adminID, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
		})

		res, err := st.svc.UpdateGroups(t.Context(), adminActor(), user.UpdateGroupsData{
			UserID:                    targetUUID,
			AddGroupIDs:               []string{writableID},
			ExpectedMembershipVersion: new(int64(0)),
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.MembershipVersion)
	})

	t.Run("no-op after version check: version not bumped", func(t *testing.T) {
		t.Parallel()

		st := updateGroupsSetup(t)
		addPolicies(t, st, []policyRow{
			{adminID, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
		})

		// Apply once so version becomes 1.
		_, err := st.svc.UpdateGroups(t.Context(), adminActor(), user.UpdateGroupsData{
			UserID:      targetUUID,
			AddGroupIDs: []string{writableID},
		})
		require.NoError(t, err)

		// Optimistic lock matches and the request is an effective no-op
		// (already a member). Version must stay at 1.
		res, err := st.svc.UpdateGroups(t.Context(), adminActor(), user.UpdateGroupsData{
			UserID:                    targetUUID,
			AddGroupIDs:               []string{writableID},
			ExpectedMembershipVersion: new(int64(1)),
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), res.MembershipVersion)

		persisted, err := st.users.GetByIdentity(t.Context(), string(domain.ProviderBasic), targetEmail)
		require.NoError(t, err)
		assert.Equal(t, int64(1), persisted.MembershipVersion)
	})
}

// TestService_UpdateGroups_DeactivatedUser verifies that PAP is orthogonal to
// user status: group membership can be updated for a deactivated user without
// any extra status check in the usecase layer.
func TestService_UpdateGroups_DeactivatedUser(t *testing.T) {
	t.Parallel()

	st := updateGroupsSetup(t)
	addPolicies(t, st, []policyRow{
		{adminID, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
	})

	// Deactivate the target user.
	_, err := st.svc.Deactivate(t.Context(), adminActor(), targetUUID)
	require.NoError(t, err)

	// UpdateGroups on a deactivated user must succeed — PAP has no status guard.
	res, err := st.svc.UpdateGroups(t.Context(), adminActor(), user.UpdateGroupsData{
		UserID:      targetUUID,
		AddGroupIDs: []string{writableID},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.MembershipVersion)
	assert.ElementsMatch(t, []string{writableID}, res.VisibleGroupIDs)

	// Verify the Casbin g-rule was persisted.
	roles, err := st.enforcer.GetRolesForUser(targetID, domain.MembershipDomain)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{domain.GroupResource("writable")}, roles)
}

// TestService_UpdateGroups_VisibleGroupIDs locks the enumeration-leak
// protection on the response: only groups the caller can read are exposed.
func TestService_UpdateGroups_VisibleGroupIDs(t *testing.T) {
	t.Parallel()

	st := updateGroupsSetup(t)
	// Admin assigns target to both groups. Actor can read+write writable but
	// has NO grant on escalate — so escalate must be filtered out of the
	// VisibleGroupIDs response (enumeration-leak protection). Note write⊇read
	// (domain.ActionGrants): any escalate grant, even Write, would reveal it.
	addPolicies(t, st, []policyRow{
		{adminID, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
		{actorID, domain.GroupResource(writableID), domain.ObjectGroup, domain.ActionRead},
		{actorID, domain.GroupResource(writableID), domain.ObjectGroup, domain.ActionWrite},
	})

	_, err := st.svc.UpdateGroups(t.Context(), adminActor(), user.UpdateGroupsData{
		UserID:      targetUUID,
		AddGroupIDs: []string{writableID, escalateID},
	})
	require.NoError(t, err)

	// Actor submits a no-op on writable only (target already a member, so no
	// escalate write is required). VisibleGroupIDs filters target's groups
	// through the actor's Group:Read scope — escalate hidden, writable revealed.
	res, err := st.svc.UpdateGroups(t.Context(), actor(), user.UpdateGroupsData{
		UserID:      targetUUID,
		AddGroupIDs: []string{writableID}, // no-op
	})
	require.NoError(t, err)
	assert.Equal(t, []string{writableID}, res.VisibleGroupIDs)
}
