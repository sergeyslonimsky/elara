package group_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/group"
)

// slugify converts a test-case name into a DNS-1123-safe label by replacing
// non-alphanumeric characters with hyphens and collapsing consecutive hyphens.
var nonAlphanumRE = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
	}
	s = strings.TrimRight(s, "-")

	return s
}

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
				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "old-name"},
				)
				require.NoError(t, err)

				return adminAuth(), group.UpdateData{
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
			name: "group not found",
			setup: func(_ *testing.T, _ testStack) (domain.AuthInfo, group.UpdateData) {
				return adminAuth(), group.UpdateData{
					Name:                    "missing-name",
					ExpectedMetadataVersion: new(int64(0)),
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
			Name:                    "concurrent-group",
			Description:             "Update A",
			ExpectedMetadataVersion: new(created.Group.MetadataVersion),
		})
		errs <- err
	}()

	go func() {
		defer wg.Done()
		_, err := st.svc.Update(ctx, adminAuth(), group.UpdateData{
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

	final, err := st.svc.Get(ctx, adminAuth(), created.Group.Name)
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
		setupGroup  func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.Manager)
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
				{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: domain.NamespaceResource("dev"),
				},
			},
		},
		{
			name:      "grant outside boundary",
			principal: "devops@example.com",
			permissions: []domain.Permission{
				{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: domain.NamespaceResource("prod"),
				},
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
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.Manager) {
				_ = enforcer.WriteTx(ctx, txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						domain.GroupResource(g.Name),
						domain.NamespaceResource("prod"),
						string(domain.ObjectNamespace),
						string(domain.ActionWrite),
					)
				})
			},
			removePerms: []domain.Permission{
				{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: domain.NamespaceResource("prod"),
				},
			},
		},
		{
			name:      "revoke within boundary",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.Manager) {
				_ = enforcer.WriteTx(ctx, txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						domain.GroupResource(g.Name),
						domain.NamespaceResource("dev"),
						string(domain.ObjectNamespace),
						string(domain.ActionWrite),
					)
				})
			},
			removePerms: []domain.Permission{
				{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: domain.NamespaceResource("dev"),
				},
			},
		},
		{
			// adminID matches the wildcard policy seeded in the test setup.
			name:      "superadmin wildcard",
			principal: adminID,
			permissions: []domain.Permission{
				{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: domain.NamespaceResource("prod"),
				},
			},
		},

		// T4.3 - union rule for members
		{
			name:      "member-add to group with outside-boundary perm",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.Manager) {
				_ = enforcer.WriteTx(ctx, txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						domain.GroupResource(g.Name),
						domain.NamespaceResource("prod"),
						string(domain.ObjectNamespace),
						string(domain.ActionWrite),
					)
				})
			},
			permissions: []domain.Permission{
				{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: domain.NamespaceResource("prod"),
				},
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
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.Manager) {
				// NOTE: pre-seeding members via bbolt no longer applies — see
				// the SSoT refactor. UpdateMembers reads from Casbin directly,
				// so cases that depend on a pre-existing member need to seed
				// the g-rule via casbin.WriteTx. TODO: revisit these cases.
				_ = enforcer.WriteTx(ctx, txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						domain.GroupResource(g.Name),
						domain.NamespaceResource("dev"),
						string(domain.ObjectNamespace),
						string(domain.ActionWrite),
					)
				})
			},
			permissions: []domain.Permission{
				{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: domain.NamespaceResource("dev"),
				},
			},
			members: []string{"newuser@example.com"},
		},
		{
			name:      "passthrough: simultaneous perm change + member change, all in boundary",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.Manager) {
				// Seed an existing member via Casbin (SSoT) so the cascade
				// check on UpdatePermissions later sees a non-empty member
				// set and exercises the actor's boundary against the post-state.
				_ = enforcer.WriteTx(ctx, txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
					return txe.AddRoleForUser(
						"olduser@example.com", domain.GroupResource(g.Name), domain.MembershipDomain,
					)
				})
			},
			permissions: []domain.Permission{
				{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: domain.NamespaceResource("dev"),
				},
			},
			members: []string{"newuser@example.com"},
		},
		{
			name:      "passthrough loophole: member-add with unchanged outside-boundary perm",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.Manager) {
				_ = enforcer.WriteTx(ctx, txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						domain.GroupResource(g.Name),
						domain.NamespaceResource("prod"),
						string(domain.ObjectNamespace),
						string(domain.ActionWrite),
					)
				})
			},
			permissions: []domain.Permission{
				{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: domain.NamespaceResource("prod"),
				},
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

			require.NoError(
				t,
				st.enforcer.WriteTx(ctx, st.txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
					_ = txe.AddPolicy(
						adminID,
						domain.DomainAll,
						string(domain.ObjectAll),
						string(domain.ActionAll),
					)
					_ = txe.AddPolicy(
						"devops@example.com",
						domain.NamespaceResource("dev"),
						string(domain.ObjectNamespace),
						string(domain.ActionWrite),
					)

					return nil
				}),
			)

			groupName := "g-" + slugify(tc.name)
			created, err := st.svc.Create(ctx, adminAuth(), group.CreateData{Name: groupName})
			require.NoError(t, err)

			if tc.setupGroup != nil {
				tc.setupGroup(ctx, created.Group, st.enforcer, st.txm)
			}

			fresh, err := st.svc.Get(ctx, adminAuth(), created.Group.Name)
			require.NoError(t, err)

			// Boundary semantics are split between UpdatePermissions and
			// UpdateMembers. We exercise both in sequence; the first one to
			// fail surfaces tc.errIs.
			// UserID is the Casbin subject; production code uses actor.UserID.
			actor := domain.AuthInfo{UserID: tc.principal, Email: tc.principal}

			_, permErr := st.svc.UpdatePermissions(ctx, actor, group.UpdatePermissionsData{
				GroupName: fresh.Group.Name,
				Add:       tc.permissions,
				Remove:    tc.removePerms,
			})
			if tc.errIs != nil && permErr != nil {
				require.ErrorIs(t, permErr, tc.errIs)

				return
			}
			require.NoError(t, permErr)

			_, memErr := st.svc.UpdateMembers(ctx, actor, group.UpdateMembersData{
				GroupName: fresh.Group.Name,
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

				created := &domain.Group{
					Name:            "sys-group",
					System:          true,
					MetadataVersion: 1,
				}
				require.NoError(t, st.repo.Create(t.Context(), created))

				return group.UpdateData{
					Name:                    "sys-group",
					ExpectedMetadataVersion: new(created.MetadataVersion),
				}
			},
			errIs: domain.ErrSystemImmutable,
		},
		{
			name: "version mismatch",
			setup: func(t *testing.T, st testStack) group.UpdateData {
				t.Helper()

				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "ver-group"},
				)
				require.NoError(t, err)

				return group.UpdateData{
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

// TestService_Update_NameImmutability verifies that Update is a metadata-only
// operation (DisplayName, Description) and does not change the group's Name.
// Name uniqueness at create time is covered by TestService_Create_NameUniqueness.
func TestService_Update_NameImmutability(t *testing.T) {
	t.Parallel()

	st := newTestStack(t)
	seedAdminWildcard(t, st)
	ctx := t.Context()

	created, err := st.svc.Create(ctx, adminAuth(), group.CreateData{Name: "alpha"})
	require.NoError(t, err)

	// Update with the same Name (lookup key) and new description succeeds; Name
	// is unchanged — renaming is not supported.
	got, err := st.svc.Update(ctx, adminAuth(), group.UpdateData{
		Name:                    "alpha",
		Description:             "updated description",
		ExpectedMetadataVersion: new(created.Group.MetadataVersion),
	})
	require.NoError(t, err)
	assert.Equal(t, "alpha", got.Name)
	assert.Equal(t, "updated description", got.Description)
}

// TestService_UpdateMembers covers the explicit add/remove delta for group
// membership: no-op semantics, anti-escalation, version conflict, validation.
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
				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "g1"},
				)
				require.NoError(t, err)

				return adminAuth(), group.UpdateMembersData{
					GroupName:              created.Group.Name,
					AddEmails:              []string{"alice@example.com"},
					ExpectedMembersVersion: new(created.Group.MembersVersion),
				}
			},
			assert: func(t *testing.T, st testStack, _ group.UpdateMembersData, got *group.UpdateMembersResult) {
				t.Helper()

				assert.Equal(t, int64(1), got.Group.MembersVersion,
					"empty -> 1 member: MembersVersion bumped once")
				assert.ElementsMatch(t, []string{"alice@example.com"}, got.VisibleMembers)
				assert.Contains(
					t,
					st.enforcer.GetMembersOfGroup(domain.GroupResource("g1")),
					"alice@example.com",
				)
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
					GroupName:              created.Group.Name,
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
				assert.Empty(t, st.enforcer.GetMembersOfGroup(domain.GroupResource("g1")))
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
					GroupName:              created.Group.Name,
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
				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "g1"},
				)
				require.NoError(t, err)

				return adminAuth(), group.UpdateMembersData{
					GroupName:              created.Group.Name,
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
				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "g1"},
				)
				require.NoError(t, err)

				return adminAuth(), group.UpdateMembersData{
					GroupName:              created.Group.Name,
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
					st.enforcer.WriteTx(
						t.Context(),
						st.txm,
						func(ctx context.Context, txe *casbin.TxEnforcer) error {
							return txe.AddPolicy(
								"devops@example.com",
								"dev",
								string(domain.ObjectNamespace),
								string(domain.ActionWrite),
							)
						},
					),
				)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name: "elevated",
					InitialPermissions: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: domain.NamespaceResource("prod"),
						},
					},
				})
				require.NoError(t, err)

				auth := domain.AuthInfo{
					UserID: "devops@example.com",
					Email:  "devops@example.com",
				}

				return auth, group.UpdateMembersData{
					GroupName:              created.Group.Name,
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
					st.enforcer.WriteTx(
						t.Context(),
						st.txm,
						func(ctx context.Context, txe *casbin.TxEnforcer) error {
							return txe.AddPolicy(
								"devops@example.com",
								"dev",
								string(domain.ObjectNamespace),
								string(domain.ActionWrite),
							)
						},
					),
				)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name:           "elevated",
					InitialMembers: []string{"alice@example.com"},
					InitialPermissions: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: domain.NamespaceResource("prod"),
						},
					},
				})
				require.NoError(t, err)

				auth := domain.AuthInfo{
					UserID: "devops@example.com",
					Email:  "devops@example.com",
				}

				return auth, group.UpdateMembersData{
					GroupName:              created.Group.Name,
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
				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "g1"},
				)
				require.NoError(t, err)

				return adminAuth(), group.UpdateMembersData{
					GroupName:              created.Group.Name,
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
					GroupName:              "missing-name",
					AddEmails:              []string{"alice@example.com"},
					ExpectedMembersVersion: new(int64(0)),
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
				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "g1"},
				)
				require.NoError(t, err)

				return adminAuth(), group.UpdatePermissionsData{
					GroupName: created.Group.Name,
					Add: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: domain.NamespaceResource("dev"),
						},
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
					GroupName: created.Group.Name,
					Add: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: domain.NamespaceResource("dev"),
						},
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
					st.enforcer.WriteTx(
						t.Context(),
						st.txm,
						func(ctx context.Context, txe *casbin.TxEnforcer) error {
							return txe.AddPolicy(
								"devops@example.com",
								"dev",
								string(domain.ObjectNamespace),
								string(domain.ActionWrite),
							)
						},
					),
				)

				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "g1"},
				)
				require.NoError(t, err)

				auth := domain.AuthInfo{
					UserID: "devops@example.com",
					Email:  "devops@example.com",
				}

				return auth, group.UpdatePermissionsData{
					GroupName: created.Group.Name,
					Add: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: domain.NamespaceResource("prod"),
						},
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
					st.enforcer.WriteTx(
						t.Context(),
						st.txm,
						func(ctx context.Context, txe *casbin.TxEnforcer) error {
							return txe.AddPolicy(
								"devops@example.com",
								domain.NamespaceResource("dev"),
								string(domain.ObjectNamespace),
								string(domain.ActionWrite),
							)
						},
					),
				)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name:           "g1",
					InitialMembers: []string{"alice@example.com"},
					InitialPermissions: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: domain.NamespaceResource("prod"),
						},
					},
				})
				require.NoError(t, err)

				auth := domain.AuthInfo{
					UserID: "devops@example.com",
					Email:  "devops@example.com",
				}

				return auth, group.UpdatePermissionsData{
					GroupName: created.Group.Name,
					Add: []domain.Permission{
						// devops holds dev, so the Add boundary passes.
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: domain.NamespaceResource("dev"),
						},
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
					st.enforcer.WriteTx(
						t.Context(),
						st.txm,
						func(ctx context.Context, txe *casbin.TxEnforcer) error {
							return txe.AddPolicy(
								"devops@example.com",
								"dev",
								string(domain.ObjectNamespace),
								string(domain.ActionWrite),
							)
						},
					),
				)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name: "g1",
					InitialPermissions: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: domain.NamespaceResource("prod"),
						},
					},
				})
				require.NoError(t, err)

				auth := domain.AuthInfo{
					UserID: "devops@example.com",
					Email:  "devops@example.com",
				}

				return auth, group.UpdatePermissionsData{
					GroupName: created.Group.Name,
					Remove: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: domain.NamespaceResource("prod"),
						},
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
				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "g1"},
				)
				require.NoError(t, err)

				return adminAuth(), group.UpdatePermissionsData{
					GroupName: created.Group.Name,
					Add: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionRead,
							Domain: domain.NamespaceResource("dev"),
						},
					},
					Remove: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionRead,
							Domain: domain.NamespaceResource("dev"),
						},
					},
					ExpectedPermissionsVersion: new(created.Group.PermissionsVersion),
				}
			},
			// Wrapped message includes Object:Action on Domain.
			wantErr: "namespace:read on namespace:dev appears in both add and remove",
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
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionRead,
							Domain: domain.NamespaceResource("dev"),
						},
					},
				})
				require.NoError(t, err)

				return adminAuth(), group.UpdatePermissionsData{
					GroupName: created.Group.Name,
					// Add: already present -> noop. Remove: absent -> noop.
					Add: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionRead,
							Domain: domain.NamespaceResource("dev"),
						},
					},
					Remove: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionRead,
							Domain: domain.NamespaceResource("staging"),
						},
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
				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "g1"},
				)
				require.NoError(t, err)

				return adminAuth(), group.UpdatePermissionsData{
					GroupName: created.Group.Name,
					Add: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionRead,
							Domain: domain.NamespaceResource("dev"),
						},
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
					GroupName: "missing-name",
					Add: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionRead,
							Domain: domain.NamespaceResource("dev"),
						},
					},
					ExpectedPermissionsVersion: new(int64(0)),
				}
			},
			wantErr: "get group",
		},
		{
			// Unlike the "cascade fail" case above (which seeds the actor's
			// policy with a bare, non-canonical domain that never matches
			// canonical Permission.Domain values and so actually fails at
			// the boundaryCheckPerms step), this case grants the actor a
			// canonical dev policy so boundaryCheckPerms passes on Add, and
			// the group's pre-existing prod perm — which is outside the
			// actor's grants — is what trips cascadeCheckPerms once the
			// group has a member.
			name: "cascade fail: actor holds added perm but not an existing one, with members",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdatePermissionsData) {
				t.Helper()

				seedAdminWildcard(t, st)

				require.NoError(
					t,
					st.enforcer.WriteTx(
						t.Context(),
						st.txm,
						func(ctx context.Context, txe *casbin.TxEnforcer) error {
							return txe.AddPolicy(
								"devops@example.com",
								domain.NamespaceResource("dev"),
								string(domain.ObjectNamespace),
								string(domain.ActionWrite),
							)
						},
					),
				)

				created, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name:           "g1",
					InitialMembers: []string{"alice@example.com"},
					InitialPermissions: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: domain.NamespaceResource("prod"),
						},
					},
				})
				require.NoError(t, err)

				auth := domain.AuthInfo{
					UserID: "devops@example.com",
					Email:  "devops@example.com",
				}

				return auth, group.UpdatePermissionsData{
					GroupName: created.Group.Name,
					Add: []domain.Permission{
						// devops holds canonical dev, so the boundary
						// check on Add passes; the cascade check then
						// fails on the group's existing prod perm.
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: domain.NamespaceResource("dev"),
						},
					},
					ExpectedPermissionsVersion: new(created.Group.PermissionsVersion),
				}
			},
			errIs: domain.ErrPermissionEscalation,
		},
		{
			name: "invalid permission assignment rejected before any mutation",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.UpdatePermissionsData) {
				t.Helper()

				seedAdminWildcard(t, st)
				created, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "g1"},
				)
				require.NoError(t, err)

				return adminAuth(), group.UpdatePermissionsData{
					GroupName: created.Group.Name,
					Add: []domain.Permission{
						// domain.ObjectPolicy has no catalog entry, so
						// validatePermissionAssignment rejects it.
						{
							Object: domain.ObjectPolicy,
							Action: domain.ActionRead,
							Domain: domain.DomainAll,
						},
					},
					ExpectedPermissionsVersion: new(created.Group.PermissionsVersion),
				}
			},
			wantErr: "not assignable",
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
