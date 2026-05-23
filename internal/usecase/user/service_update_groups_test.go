package user_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/user"
)

// TestService_UpdateGroups exercises the per-delta authorization and Casbin
// g-rule sync end-to-end against a real bbolt + Enforcer stack. We exercise
// the integration helper (not the mocked store) because every assertion
// hinges on whether Casbin actually reflects the requested mutation.
func TestService_UpdateGroups(t *testing.T) {
	t.Parallel()

	const (
		adminEmail  = "admin@example.com"
		actorEmail  = "actor@example.com"
		targetEmail = "target@example.com"
		writableID  = "writable-group"
		escalateID  = "escalate-group"
	)

	admin := domain.AuthInfo{Email: adminEmail}
	actor := domain.AuthInfo{Email: actorEmail}

	type setupOut struct {
		sut      *user.Service
		store    *bbolt.Store
		enforcer *casbin.Enforcer
	}

	// setup seeds two groups (writable, escalate), creates the target user,
	// and returns the SUT plus handles for per-test grants.
	setup := func(t *testing.T) setupOut {
		t.Helper()

		sut, store, enforcer, users := setupServiceReal(t)
		ctx := t.Context()

		require.NoError(
			t,
			users.Upsert(ctx, &domain.User{Email: targetEmail, Name: "Target", Provider: domain.ProviderBasicAuth}),
		)
		require.NoError(
			t,
			users.Upsert(ctx, &domain.User{Email: actorEmail, Name: "Actor", Provider: domain.ProviderBasicAuth}),
		)
		require.NoError(
			t,
			users.Upsert(ctx, &domain.User{Email: adminEmail, Name: "Admin", Provider: domain.ProviderBasicAuth}),
		)

		groups := bbolt.NewGroupRepo(store)
		require.NoError(t, groups.Create(ctx, &domain.Group{ID: writableID, Name: "writable"}))
		require.NoError(t, groups.Create(ctx, &domain.Group{ID: escalateID, Name: "escalate"}))

		return setupOut{sut: sut, store: store, enforcer: enforcer}
	}

	grantPolicies := func(t *testing.T, s setupOut, rules [][]string) {
		t.Helper()

		txm := bbolt.NewTxManager(s.store.DB())
		require.NoError(t, s.enforcer.WriteTx(t.Context(), txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
			for _, r := range rules {
				if err := txe.AddPolicy(r[0], r[1], r[2], r[3]); err != nil {
					return err
				}
			}

			return nil
		}))
	}

	t.Run("happy path: admin grants two empty groups", func(t *testing.T) {
		t.Parallel()

		s := setup(t)
		// Wildcard grant: passes per-group write and (vacuous) anti-escalation.
		grantPolicies(t, s, [][]string{{adminEmail, domain.DomainAll, domain.ObjectAll, domain.ActionAll}})

		result, err := s.sut.UpdateGroups(t.Context(), admin, user.UpdateGroupsData{
			Email: targetEmail, GroupIDs: []string{writableID, escalateID},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{writableID, escalateID}, result.GroupIDs)

		roles, err := s.enforcer.GetRolesForUser(targetEmail, domain.MembershipDomain)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"group:writable", "group:escalate"}, roles)
	})

	t.Run("forbidden when actor lacks ObjectGroup:Write on a delta", func(t *testing.T) {
		t.Parallel()

		s := setup(t)
		// Actor can write to `writable` only; attempt also touches `escalate`.
		grantPolicies(t, s, [][]string{
			{actorEmail, "group:" + writableID, domain.ObjectGroup, domain.ActionWrite},
		})

		_, err := s.sut.UpdateGroups(t.Context(), actor, user.UpdateGroupsData{
			Email: targetEmail, GroupIDs: []string{writableID, escalateID},
		})
		require.ErrorIs(t, err, domain.ErrForbidden)

		// Transaction rolled back — no memberships persisted.
		roles, err := s.enforcer.GetRolesForUser(targetEmail, domain.MembershipDomain)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("escalation blocked when actor lacks a group permission", func(t *testing.T) {
		t.Parallel()

		s := setup(t)
		// Seed `escalate` with a config-write permission, then grant the actor
		// ObjectGroup:Write on that group (so the per-delta check passes) but
		// withhold the config-write perm (so anti-escalation trips).
		grantPolicies(t, s, [][]string{
			{"group:escalate", "ns-a", domain.ObjectConfig, domain.ActionWrite},
			{actorEmail, "group:" + escalateID, domain.ObjectGroup, domain.ActionWrite},
		})

		_, err := s.sut.UpdateGroups(t.Context(), actor, user.UpdateGroupsData{
			Email: targetEmail, GroupIDs: []string{escalateID},
		})
		require.ErrorIs(t, err, domain.ErrPermissionEscalation)
	})

	t.Run("removal succeeds with only ObjectGroup:Write (no anti-escalation)", func(t *testing.T) {
		t.Parallel()

		s := setup(t)
		// Bootstrap: admin seeds `escalate` membership for the target, while
		// also giving `escalate` a config-write permission. The actor then
		// removes the target — they need ObjectGroup:Write on `escalate` but
		// crucially NOT the config-write permission, because removal narrows
		// (not widens) the target's effective permissions.
		grantPolicies(t, s, [][]string{
			{adminEmail, domain.DomainAll, domain.ObjectAll, domain.ActionAll},
			{"group:escalate", "ns-a", domain.ObjectConfig, domain.ActionWrite},
			{actorEmail, "group:" + escalateID, domain.ObjectGroup, domain.ActionWrite},
		})

		_, err := s.sut.UpdateGroups(t.Context(), admin, user.UpdateGroupsData{
			Email: targetEmail, GroupIDs: []string{escalateID},
		})
		require.NoError(t, err)

		// Now remove via the lower-privileged actor (no config:write perm).
		_, err = s.sut.UpdateGroups(t.Context(), actor, user.UpdateGroupsData{
			Email: targetEmail, GroupIDs: []string{},
		})
		require.NoError(t, err)

		roles, err := s.enforcer.GetRolesForUser(targetEmail, domain.MembershipDomain)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("idempotent no-op when desired equals current", func(t *testing.T) {
		t.Parallel()

		s := setup(t)
		grantPolicies(t, s, [][]string{{adminEmail, domain.DomainAll, domain.ObjectAll, domain.ActionAll}})

		// Establish the initial state.
		_, err := s.sut.UpdateGroups(t.Context(), admin, user.UpdateGroupsData{
			Email: targetEmail, GroupIDs: []string{writableID},
		})
		require.NoError(t, err)

		// Re-issue the same request: no diff, no error, no duplicate g-rule.
		_, err = s.sut.UpdateGroups(t.Context(), admin, user.UpdateGroupsData{
			Email: targetEmail, GroupIDs: []string{writableID},
		})
		require.NoError(t, err)

		roles, err := s.enforcer.GetRolesForUser(targetEmail, domain.MembershipDomain)
		require.NoError(t, err)
		assert.Equal(t, []string{"group:writable"}, roles)
	})
}
