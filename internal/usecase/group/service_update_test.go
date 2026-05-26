package group_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/group"
)

// Authorization for GroupService.Update is enforced in the handler layer
// (EL-4 M9). These tests cover only the business logic remaining in the
// usecase: store/Casbin mutations, version conflicts, immutability and the
// PDP boundary checks for permission/member deltas. The new Update signature
// takes a domain.AuthInfo carrying the caller identity used by the PDP.

func TestService_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// setup creates the group (and any extra Casbin state) the test
		// operates on. It returns the data the caller will pass to Update.
		setup   func(t *testing.T, st testStack) (auth domain.AuthInfo, data group.UpdateData)
		assert  func(t *testing.T, st testStack, got *domain.Group)
		errIs   error
		wantErr string
	}{
		{
			name: "name unchanged -> store update only, no Casbin mutation",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "old-name"})
				require.NoError(t, err)

				return adminAuth(), group.UpdateData{
					ID:                      created.Group.ID,
					Name:                    "old-name",
					Description:             "desc",
					ExpectedMetadataVersion: new(created.Group.MetadataVersion),
				}
			},
			assert: func(t *testing.T, st testStack, got *domain.Group) {
				t.Helper()

				assert.Equal(t, "old-name", got.Name)
				assert.Equal(t, "desc", got.Description)
				assert.Empty(t, st.enforcer.GetGroupingPolicy(),
					"unchanged name must not mutate Casbin grouping policy")
			},
		},
		{
			name: "rename rewrites memberships under the new prefix",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "old-name"})
				require.NoError(t, err)

				_, err = st.svc.UpdateMembers(t.Context(), adminAuth(), group.UpdateMembersData{
					GroupID:   created.Group.ID,
					AddEmails: []string{"user1@example.com"},
				})
				require.NoError(t, err)

				oldSub := casbin.GroupSubject("old-name")
				require.NotEmpty(t, st.enforcer.GetMembersOfGroup(oldSub),
					"precondition: membership rule exists under old name")

				return adminAuth(), group.UpdateData{
					ID:                      created.Group.ID,
					Name:                    "new-name",
					ExpectedMetadataVersion: new(created.Group.MetadataVersion),
				}
			},
			assert: func(t *testing.T, st testStack, got *domain.Group) {
				t.Helper()

				assert.Equal(t, "new-name", got.Name)

				oldSub := casbin.GroupSubject("old-name")
				newSub := casbin.GroupSubject("new-name")
				assert.Empty(t, st.enforcer.GetMembersOfGroup(oldSub),
					"old subject must have no remaining members")
				assert.Contains(t, st.enforcer.GetMembersOfGroup(newSub), "user1@example.com",
					"new subject must inherit memberships")
			},
		},
		{
			name: "rename carries p-rules from old subject to new subject",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "old-name"})
				require.NoError(t, err)

				perm := domain.Permission{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: "dev",
				}
				_, err = st.svc.UpdatePermissions(t.Context(), adminAuth(), group.UpdatePermissionsData{
					GroupID: created.Group.ID,
					Add:     []domain.Permission{perm},
				})
				require.NoError(t, err)

				return adminAuth(), group.UpdateData{
					ID:                      created.Group.ID,
					Name:                    "new-name",
					ExpectedMetadataVersion: new(created.Group.MetadataVersion),
				}
			},
			assert: func(t *testing.T, st testStack, got *domain.Group) {
				t.Helper()

				assert.Equal(t, "new-name", got.Name)

				oldSub := casbin.GroupSubject("old-name")
				newSub := casbin.GroupSubject("new-name")

				var oldCount, newCount int
				for _, rule := range st.enforcer.GetPolicy() {
					switch rule[0] {
					case oldSub:
						oldCount++
					case newSub:
						newCount++
					}
				}
				assert.Zero(t, oldCount, "p-rules must not remain under old subject after rename")
				assert.Equal(t, 1, newCount, "p-rules must be rebound to new subject after rename")
			},
		},
		{
			name: "group not found",
			setup: func(_ *testing.T, _ testStack) (domain.AuthInfo, group.UpdateData) {
				return adminAuth(), group.UpdateData{
					ID:                      "missing-id",
					Name:                    "any-name",
					ExpectedMetadataVersion: int64Ptr(0),
				}
			},
			wantErr: "get group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)
			auth, data := tt.setup(t, st)

			got, err := st.svc.Update(t.Context(), auth, data)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			if tt.assert != nil {
				tt.assert(t, st, got)
			}
		})
	}
}

func TestService_Update_ConcurrentVersionConflict(t *testing.T) {
	t.Parallel()

	st := newTestStack(t)
	seedAdminWildcard(t, st)
	ctx := t.Context()

	created, err := st.svc.Create(ctx, adminAuth(), group.CreateData{Name: "concurrent-group"})
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err := st.svc.Update(ctx, adminAuth(), group.UpdateData{
			ID:                      created.Group.ID,
			Name:                    "concurrent-group",
			Description:             "Update A",
			ExpectedMetadataVersion: new(created.Group.MetadataVersion),
		})
		errs <- err
	}()

	go func() {
		defer wg.Done()
		_, err := st.svc.Update(ctx, adminAuth(), group.UpdateData{
			ID:                      created.Group.ID,
			Name:                    "concurrent-group",
			Description:             "Update B",
			ExpectedMetadataVersion: new(created.Group.MetadataVersion),
		})
		errs <- err
	}()

	wg.Wait()
	close(errs)

	var successCount, conflictCount int
	for err := range errs {
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, domain.ErrVersionConflict):
			conflictCount++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}

	assert.Equal(t, 1, successCount, "exactly one update should succeed")
	assert.Equal(t, 1, conflictCount, "exactly one update should fail with version conflict")

	final, err := st.svc.Get(ctx, adminAuth(), created.Group.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Group.MetadataVersion+1, final.Group.MetadataVersion,
		"metadata version should increment exactly once")

	if final.Group.Description != "Update A" && final.Group.Description != "Update B" {
		t.Errorf("unexpected final description: %s", final.Group.Description)
	}
}

func TestService_Update_Boundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		principal string
		// setupGroup customises the initial group state (members, existing
		// Casbin policies) before Update is invoked.
		setupGroup  func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager)
		permissions []domain.Permission // Add list passed to UpdatePermissions
		removePerms []domain.Permission // Remove list passed to UpdatePermissions
		members     []string
		errIs       error
	}{
		// T4.2 - permissions boundary
		{
			name:      "grant within boundary",
			principal: "devops@example.com",
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
			},
		},
		{
			name:      "grant outside boundary",
			principal: "devops@example.com",
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
			},
			errIs: domain.ErrPermissionEscalation,
		},
		{
			// Permission removal narrows the group's grants and therefore
			// does not require anti-escalation — explicit-delta semantics.
			// The actor revokes a perm they do not personally hold; before
			// the SSoT split this returned ErrPermissionEscalation, now
			// it succeeds (group has no members, so no cascade either).
			name:      "revoke outside boundary succeeds without escalation check",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				_ = enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						casbin.GroupSubject(g.Name),
						"prod",
						domain.ObjectNamespace,
						domain.ActionWrite,
					)
				})
			},
			removePerms: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
			},
		},
		{
			name:      "revoke within boundary",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				_ = enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(casbin.GroupSubject(g.Name), "dev", domain.ObjectNamespace, domain.ActionWrite)
				})
			},
			removePerms: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
			},
		},
		{
			name:      "superadmin wildcard",
			principal: "admin@example.com",
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
			},
		},

		// T4.3 - union rule for members
		{
			name:      "member-add to group with outside-boundary perm",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				_ = enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						casbin.GroupSubject(g.Name),
						"prod",
						domain.ObjectNamespace,
						domain.ActionWrite,
					)
				})
			},
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
			},
			members: []string{"newuser@example.com"},
			errIs:   domain.ErrPermissionEscalation,
		},
		// "member-remove from group with outside-boundary perm" was removed:
		// the SSoT split clarified that membership removal narrows and does
		// not require anti-escalation (see UpdateUserGroups for the same
		// stance). The old combined Update intentionally over-checked.
		{
			name:      "member change with all perms in boundary",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				// NOTE: pre-seeding members via bbolt no longer applies — see
				// the SSoT refactor. UpdateMembers reads from Casbin directly,
				// so cases that depend on a pre-existing member need to seed
				// the g-rule via casbin.WriteTx. TODO: revisit these cases.
				_ = enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(casbin.GroupSubject(g.Name), "dev", domain.ObjectNamespace, domain.ActionWrite)
				})
			},
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
			},
			members: []string{"newuser@example.com"},
		},
		{
			name:      "passthrough: simultaneous perm change + member change, all in boundary",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				// Seed an existing member via Casbin (SSoT) so the cascade
				// check on UpdatePermissions later sees a non-empty member
				// set and exercises the actor's boundary against the post-state.
				_ = enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddRoleForUser(
						"olduser@example.com", casbin.GroupSubject(g.Name), domain.MembershipDomain,
					)
				})
			},
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
			},
			members: []string{"newuser@example.com"},
		},
		{
			name:      "passthrough loophole: member-add with unchanged outside-boundary perm",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				_ = enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						casbin.GroupSubject(g.Name),
						"prod",
						domain.ObjectNamespace,
						domain.ActionWrite,
					)
				})
			},
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
			},
			members: []string{"hacker@example.com"},
			errIs:   domain.ErrPermissionEscalation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)
			ctx := t.Context()

			require.NoError(t, st.enforcer.WriteTx(ctx, st.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
				_ = txe.AddPolicy("admin@example.com", domain.DomainAll, domain.ObjectAll, domain.ActionAll)
				_ = txe.AddPolicy("devops@example.com", "dev", domain.ObjectNamespace, domain.ActionWrite)

				return nil
			}))

			groupName := "group-" + tc.name
			created, err := st.svc.Create(ctx, adminAuth(), group.CreateData{Name: groupName})
			require.NoError(t, err)

			if tc.setupGroup != nil {
				tc.setupGroup(ctx, created.Group, st.enforcer, st.txm)
			}

			fresh, err := st.svc.Get(ctx, adminAuth(), created.Group.ID)
			require.NoError(t, err)

			// Boundary semantics are split between UpdatePermissions and
			// UpdateMembers. We exercise both in sequence; the first one to
			// fail surfaces tc.errIs.
			actor := domain.AuthInfo{Email: tc.principal}

			_, permErr := st.svc.UpdatePermissions(ctx, actor, group.UpdatePermissionsData{
				GroupID: fresh.Group.ID,
				Add:     tc.permissions,
				Remove:  tc.removePerms,
			})
			if tc.errIs != nil && permErr != nil {
				require.ErrorIs(t, permErr, tc.errIs)

				return
			}
			require.NoError(t, permErr)

			_, memErr := st.svc.UpdateMembers(ctx, actor, group.UpdateMembersData{
				GroupID:   fresh.Group.ID,
				AddEmails: tc.members,
			})
			if tc.errIs != nil {
				require.ErrorIs(t, memErr, tc.errIs)

				return
			}
			require.NoError(t, memErr)
		})
	}
}

func TestService_Update_ImmutabilityAndVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(t *testing.T, st testStack) group.UpdateData
		errIs error
	}{
		{
			name: "system group",
			setup: func(t *testing.T, st testStack) group.UpdateData {
				t.Helper()

				created := &domain.Group{ID: "sys-group-id", Name: "sys-group", System: true, MetadataVersion: 1}
				require.NoError(t, st.repo.Create(t.Context(), created))

				v := created.MetadataVersion

				return group.UpdateData{
					ID:                      created.ID,
					Name:                    "sys-group",
					ExpectedMetadataVersion: &v,
				}
			},
			errIs: domain.ErrSystemImmutable,
		},
		{
			name: "version mismatch",
			setup: func(t *testing.T, st testStack) group.UpdateData {
				t.Helper()

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "ver-group"})
				require.NoError(t, err)

				return group.UpdateData{
					ID:                      created.Group.ID,
					Name:                    "ver-group",
					ExpectedMetadataVersion: new(created.Group.MetadataVersion + 99),
				}
			},
			errIs: domain.ErrVersionConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)
			seedAdminWildcard(t, st)

			data := tt.setup(t, st)

			_, err := st.svc.Update(t.Context(), adminAuth(), data)
			require.ErrorIs(t, err, tt.errIs)
		})
	}
}

// TestService_Update_NameUniqueness verifies the rename guard against an
// existing group's name. Self-rename (same name) and free renames must
// succeed; collision with a different group fails with ErrAlreadyExists.
func TestService_Update_NameUniqueness(t *testing.T) {
	t.Parallel()

	st := newTestStack(t)
	seedAdminWildcard(t, st)
	ctx := t.Context()

	first, err := st.svc.Create(ctx, adminAuth(), group.CreateData{Name: "alpha"})
	require.NoError(t, err)
	second, err := st.svc.Create(ctx, adminAuth(), group.CreateData{Name: "beta"})
	require.NoError(t, err)

	_, err = st.svc.Update(ctx, adminAuth(), group.UpdateData{
		ID:                      second.Group.ID,
		Name:                    "alpha",
		ExpectedMetadataVersion: new(second.Group.MetadataVersion),
	})
	require.ErrorIs(t, err, domain.ErrAlreadyExists)
	require.ErrorContains(t, err, `group "alpha"`)

	// Self-rename to the current name is a no-op for uniqueness (skipped).
	_, err = st.svc.Update(ctx, adminAuth(), group.UpdateData{
		ID:                      first.Group.ID,
		Name:                    "alpha",
		Description:             "updated-desc",
		ExpectedMetadataVersion: new(first.Group.MetadataVersion),
	})
	require.NoError(t, err)
}

// TestService_UpdateMembers covers the explicit add/remove delta for group
// membership: no-op semantics, anti-escalation, version conflict, validation.
//
//nolint:gocognit // table-driven integration test
func TestService_UpdateMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// setup prepares the group and any pre-existing memberships /
		// Casbin policies. Returns the actor and the delta to apply.
		setup   func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateMembersData)
		assert  func(t *testing.T, st testStack, in group.UpdateMembersData, got *group.UpdateMembersResult)
		errIs   error
		wantErr string
	}{
		{
			name: "happy add bumps MembersVersion and creates g-rule",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateMembersData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "g1"})
				require.NoError(t, err)

				return adminAuth(), group.UpdateMembersData{
					GroupID:                created.Group.ID,
					AddEmails:              []string{"alice@example.com"},
					ExpectedMembersVersion: new(created.Group.MembersVersion),
				}
			},
			assert: func(t *testing.T, st testStack, _ group.UpdateMembersData, got *group.UpdateMembersResult) {
				t.Helper()

				assert.Equal(t, int64(1), got.Group.MembersVersion,
					"empty -> 1 member: MembersVersion bumped once")
				assert.ElementsMatch(t, []string{"alice@example.com"}, got.VisibleMembers)
				assert.Contains(t, st.enforcer.GetMembersOfGroup(casbin.GroupSubject("g1")), "alice@example.com")
			},
		},
		{
			name: "happy remove bumps MembersVersion and deletes g-rule",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateMembersData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name:           "g1",
					InitialMembers: []string{"alice@example.com"},
				})
				require.NoError(t, err)

				return adminAuth(), group.UpdateMembersData{
					GroupID:                created.Group.ID,
					RemoveEmails:           []string{"alice@example.com"},
					ExpectedMembersVersion: new(created.Group.MembersVersion),
				}
			},
			assert: func(t *testing.T, st testStack, _ group.UpdateMembersData, got *group.UpdateMembersResult) {
				t.Helper()

				// Initial members in Create bumped MembersVersion to 1;
				// remove bumps it once more to 2.
				assert.Equal(t, int64(2), got.Group.MembersVersion)
				assert.Empty(t, got.VisibleMembers)

				// DeleteUser cache: g-rule removed from persistence; resync
				// to confirm at the persistence layer.
				require.NoError(t, st.enforcer.LoadPolicy())
				assert.Empty(t, st.enforcer.GetMembersOfGroup(casbin.GroupSubject("g1")))
			},
		},
		{
			name: "add of existing email is a no-op (no version bump)",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateMembersData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name:           "g1",
					InitialMembers: []string{"alice@example.com"},
				})
				require.NoError(t, err)

				return adminAuth(), group.UpdateMembersData{
					GroupID:                created.Group.ID,
					AddEmails:              []string{"alice@example.com"},
					ExpectedMembersVersion: new(created.Group.MembersVersion),
				}
			},
			assert: func(t *testing.T, _ testStack, _ group.UpdateMembersData, got *group.UpdateMembersResult) {
				t.Helper()

				// Initial members set MembersVersion=1; no-op must keep it.
				assert.Equal(t, int64(1), got.Group.MembersVersion,
					"no effective delta -> no version bump")
			},
		},
		{
			name: "remove of absent email is a no-op (no version bump)",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateMembersData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "g1"})
				require.NoError(t, err)

				return adminAuth(), group.UpdateMembersData{
					GroupID:                created.Group.ID,
					RemoveEmails:           []string{"ghost@example.com"},
					ExpectedMembersVersion: new(created.Group.MembersVersion),
				}
			},
			assert: func(t *testing.T, _ testStack, _ group.UpdateMembersData, got *group.UpdateMembersResult) {
				t.Helper()

				assert.Zero(t, got.Group.MembersVersion, "no effective delta -> no version bump")
			},
		},
		{
			name: "same email in add and remove is rejected with ValidationError",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateMembersData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "g1"})
				require.NoError(t, err)

				return adminAuth(), group.UpdateMembersData{
					GroupID:                created.Group.ID,
					AddEmails:              []string{"alice@example.com"},
					RemoveEmails:           []string{"alice@example.com"},
					ExpectedMembersVersion: new(created.Group.MembersVersion),
				}
			},
			// Wrapped message includes the offending email.
			wantErr: `"alice@example.com" appears in both add and remove`,
		},
		{
			name: "anti-escalation: actor lacks perms the group holds",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateMembersData) {
				t.Helper()

				seedAdminWildcard(t, st)

				// devops holds dev-only Namespace:Write. We create the group
				// as admin and seed it with a prod perm - adding a member
				// would grant prod transitively, which devops can't authorize.
				require.NoError(
					t,
					st.enforcer.WriteTx(t.Context(), st.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
						return txe.AddPolicy("devops@example.com", "dev", domain.ObjectNamespace, domain.ActionWrite)
					}),
				)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name: "elevated",
					InitialPermissions: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
					},
				})
				require.NoError(t, err)

				return domain.AuthInfo{Email: "devops@example.com"}, group.UpdateMembersData{
					GroupID:                created.Group.ID,
					AddEmails:              []string{"newcomer@example.com"},
					ExpectedMembersVersion: new(created.Group.MembersVersion),
				}
			},
			errIs: domain.ErrPermissionEscalation,
		},
		{
			name: "remove-only path skips anti-escalation check",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateMembersData) {
				t.Helper()

				seedAdminWildcard(t, st)

				// devops holds dev-only Namespace:Write but the group has prod.
				// Removing alice narrows, so the escalation check is bypassed.
				require.NoError(
					t,
					st.enforcer.WriteTx(t.Context(), st.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
						return txe.AddPolicy("devops@example.com", "dev", domain.ObjectNamespace, domain.ActionWrite)
					}),
				)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name:           "elevated",
					InitialMembers: []string{"alice@example.com"},
					InitialPermissions: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
					},
				})
				require.NoError(t, err)

				return domain.AuthInfo{Email: "devops@example.com"}, group.UpdateMembersData{
					GroupID:                created.Group.ID,
					RemoveEmails:           []string{"alice@example.com"},
					ExpectedMembersVersion: new(created.Group.MembersVersion),
				}
			},
			assert: func(t *testing.T, _ testStack, _ group.UpdateMembersData, got *group.UpdateMembersResult) {
				t.Helper()

				// Create bumped MembersVersion to 1; remove bumps to 2.
				assert.Equal(t, int64(2), got.Group.MembersVersion)
			},
		},
		{
			name: "version mismatch fails with ErrVersionConflict",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdateMembersData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "g1"})
				require.NoError(t, err)

				return adminAuth(), group.UpdateMembersData{
					GroupID:                created.Group.ID,
					AddEmails:              []string{"alice@example.com"},
					ExpectedMembersVersion: new(created.Group.MembersVersion + 99),
				}
			},
			errIs: domain.ErrVersionConflict,
		},
		{
			name: "group not found wrapped",
			setup: func(_ *testing.T, _ testStack) (domain.AuthInfo, group.UpdateMembersData) {
				return adminAuth(), group.UpdateMembersData{
					GroupID:                "missing-id",
					AddEmails:              []string{"alice@example.com"},
					ExpectedMembersVersion: int64Ptr(0),
				}
			},
			wantErr: "get group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)
			auth, data := tt.setup(t, st)

			got, err := st.svc.UpdateMembers(t.Context(), auth, data)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			if tt.assert != nil {
				tt.assert(t, st, data, got)
			}
		})
	}
}

// TestService_UpdatePermissions covers the explicit add/remove delta for the
// group's permission set: boundary check, cascade check on member-bearing
// groups, no-op semantics, version conflict, validation.
//
//nolint:gocognit // table-driven integration test
func TestService_UpdatePermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// setup prepares the group, any pre-existing p-rules / members,
		// and the actor's own permissions. Returns the actor and the
		// delta to apply.
		setup   func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdatePermissionsData)
		assert  func(t *testing.T, st testStack, in group.UpdatePermissionsData, got *group.UpdatePermissionsResult)
		errIs   error
		wantErr string
	}{
		{
			name: "happy add (no members) skips cascade, bumps PermissionsVersion",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdatePermissionsData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "g1"})
				require.NoError(t, err)

				return adminAuth(), group.UpdatePermissionsData{
					GroupID: created.Group.ID,
					Add: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
					},
					ExpectedPermissionsVersion: new(created.Group.PermissionsVersion),
				}
			},
			assert: func(t *testing.T, _ testStack, in group.UpdatePermissionsData, got *group.UpdatePermissionsResult) {
				t.Helper()

				assert.Equal(t, int64(1), got.Group.PermissionsVersion)
				assert.ElementsMatch(t, in.Add, got.Permissions)
			},
		},
		{
			// When the group has members, the cascade check runs: the actor
			// must hold every permission the group will hold post-update.
			// Admin's wildcard covers any post-state.
			name: "happy add (with members) runs cascade and passes for wildcard actor",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdatePermissionsData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name:           "g1",
					InitialMembers: []string{"alice@example.com"},
				})
				require.NoError(t, err)

				return adminAuth(), group.UpdatePermissionsData{
					GroupID: created.Group.ID,
					Add: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
					},
					ExpectedPermissionsVersion: new(created.Group.PermissionsVersion),
				}
			},
			assert: func(t *testing.T, _ testStack, in group.UpdatePermissionsData, got *group.UpdatePermissionsResult) {
				t.Helper()

				assert.Equal(t, int64(1), got.Group.PermissionsVersion)
				assert.ElementsMatch(t, in.Add, got.Permissions)
			},
		},
		{
			name: "boundary fail: actor lacks added perm",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdatePermissionsData) {
				t.Helper()

				seedAdminWildcard(t, st)

				// devops holds only dev:Namespace:Write - granting prod escalates.
				require.NoError(
					t,
					st.enforcer.WriteTx(t.Context(), st.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
						return txe.AddPolicy("devops@example.com", "dev", domain.ObjectNamespace, domain.ActionWrite)
					}),
				)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "g1"})
				require.NoError(t, err)

				return domain.AuthInfo{Email: "devops@example.com"}, group.UpdatePermissionsData{
					GroupID: created.Group.ID,
					Add: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
					},
					ExpectedPermissionsVersion: new(created.Group.PermissionsVersion),
				}
			},
			errIs: domain.ErrPermissionEscalation,
		},
		{
			// Cascade: actor holds every Add perm (boundary passes) but the
			// post-state set contains an existing pre-seeded perm the actor
			// does not hold - and the group has a member, so the change
			// cascades.
			name: "cascade fail: actor lacks an existing perm and group has members",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdatePermissionsData) {
				t.Helper()

				seedAdminWildcard(t, st)

				// devops holds dev:Namespace:Write only. The group will be
				// pre-seeded by admin with a prod perm + a member.
				require.NoError(
					t,
					st.enforcer.WriteTx(t.Context(), st.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
						return txe.AddPolicy("devops@example.com", "dev", domain.ObjectNamespace, domain.ActionWrite)
					}),
				)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name:           "g1",
					InitialMembers: []string{"alice@example.com"},
					InitialPermissions: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
					},
				})
				require.NoError(t, err)

				return domain.AuthInfo{Email: "devops@example.com"}, group.UpdatePermissionsData{
					GroupID: created.Group.ID,
					Add: []domain.Permission{
						// devops holds dev, so the Add boundary passes.
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
					},
					ExpectedPermissionsVersion: new(created.Group.PermissionsVersion),
				}
			},
			errIs: domain.ErrPermissionEscalation,
		},
		{
			// Remove narrows - boundary on Remove is not checked. Without
			// members, cascade is skipped. So an actor without the perm can
			// still revoke it.
			name: "remove without members skips boundary and cascade",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdatePermissionsData) {
				t.Helper()

				seedAdminWildcard(t, st)

				require.NoError(
					t,
					st.enforcer.WriteTx(t.Context(), st.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
						return txe.AddPolicy("devops@example.com", "dev", domain.ObjectNamespace, domain.ActionWrite)
					}),
				)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name: "g1",
					InitialPermissions: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
					},
				})
				require.NoError(t, err)

				return domain.AuthInfo{Email: "devops@example.com"}, group.UpdatePermissionsData{
					GroupID: created.Group.ID,
					Remove: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
					},
					ExpectedPermissionsVersion: new(created.Group.PermissionsVersion),
				}
			},
			assert: func(t *testing.T, _ testStack, _ group.UpdatePermissionsData, got *group.UpdatePermissionsResult) {
				t.Helper()

				// Initial perms bumped PermissionsVersion to 1; remove bumps to 2.
				assert.Equal(t, int64(2), got.Group.PermissionsVersion)
				assert.Empty(t, got.Permissions)
			},
		},
		{
			name: "same perm in add and remove returns ValidationError",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdatePermissionsData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "g1"})
				require.NoError(t, err)

				return adminAuth(), group.UpdatePermissionsData{
					GroupID: created.Group.ID,
					Add: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
					},
					Remove: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
					},
					ExpectedPermissionsVersion: new(created.Group.PermissionsVersion),
				}
			},
			// Wrapped message includes Object:Action on Domain.
			wantErr: "namespace:read on dev appears in both add and remove",
		},
		{
			// Both Add of an already-present perm AND Remove of an absent
			// perm are no-ops. No effective delta -> no version bump.
			name: "no-op delta does not bump PermissionsVersion",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdatePermissionsData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name: "g1",
					InitialPermissions: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
					},
				})
				require.NoError(t, err)

				return adminAuth(), group.UpdatePermissionsData{
					GroupID: created.Group.ID,
					// Add: already present -> noop. Remove: absent -> noop.
					Add: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
					},
					Remove: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "staging"},
					},
					ExpectedPermissionsVersion: new(created.Group.PermissionsVersion),
				}
			},
			assert: func(t *testing.T, _ testStack, _ group.UpdatePermissionsData, got *group.UpdatePermissionsResult) {
				t.Helper()

				// Initial perm set PermissionsVersion=1; no-op leaves it.
				assert.Equal(t, int64(1), got.Group.PermissionsVersion)
			},
		},
		{
			name: "version mismatch fails with ErrVersionConflict",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdatePermissionsData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "g1"})
				require.NoError(t, err)

				return adminAuth(), group.UpdatePermissionsData{
					GroupID: created.Group.ID,
					Add: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
					},
					ExpectedPermissionsVersion: new(created.Group.PermissionsVersion + 99),
				}
			},
			errIs: domain.ErrVersionConflict,
		},
		{
			name: "group not found wrapped",
			setup: func(_ *testing.T, _ testStack) (domain.AuthInfo, group.UpdatePermissionsData) {
				return adminAuth(), group.UpdatePermissionsData{
					GroupID: "missing-id",
					Add: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
					},
					ExpectedPermissionsVersion: int64Ptr(0),
				}
			},
			wantErr: "get group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)
			auth, data := tt.setup(t, st)

			got, err := st.svc.UpdatePermissions(t.Context(), auth, data)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			if tt.assert != nil {
				tt.assert(t, st, data, got)
			}
		})
	}
}
