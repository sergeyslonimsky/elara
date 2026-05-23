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
				created, err := st.svc.Create(t.Context(), adminAuth(), "old-name")
				require.NoError(t, err)

				return adminAuth(), group.UpdateData{
					ID:          created.ID,
					Name:        "old-name",
					Description: "desc",
					Version:     created.Version,
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
				created, err := st.svc.Create(t.Context(), adminAuth(), "old-name")
				require.NoError(t, err)

				_, err = st.svc.UpdateMembers(t.Context(), adminAuth(), group.UpdateMembersData{
					GroupID: created.ID,
					Members: []string{"user1@example.com"},
				})
				require.NoError(t, err)

				oldSub := casbin.GroupSubject("old-name")
				require.NotEmpty(t, st.enforcer.GetMembersOfGroup(oldSub),
					"precondition: membership rule exists under old name")

				return adminAuth(), group.UpdateData{
					ID:      created.ID,
					Name:    "new-name",
					Version: created.Version,
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
				created, err := st.svc.Create(t.Context(), adminAuth(), "old-name")
				require.NoError(t, err)

				perm := domain.Permission{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: "dev",
				}
				_, err = st.svc.UpdatePermissions(t.Context(), adminAuth(), group.UpdatePermissionsData{
					GroupID:     created.ID,
					Permissions: []domain.Permission{perm},
				})
				require.NoError(t, err)

				return adminAuth(), group.UpdateData{
					ID:      created.ID,
					Name:    "new-name",
					Version: created.Version,
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
					ID:      "missing-id",
					Name:    "any-name",
					Version: 0,
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

	created, err := st.svc.Create(ctx, adminAuth(), "concurrent-group")
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err := st.svc.Update(ctx, adminAuth(), group.UpdateData{
			ID:          created.ID,
			Name:        "concurrent-group",
			Description: "Update A",
			Version:     created.Version,
		})
		errs <- err
	}()

	go func() {
		defer wg.Done()
		_, err := st.svc.Update(ctx, adminAuth(), group.UpdateData{
			ID:          created.ID,
			Name:        "concurrent-group",
			Description: "Update B",
			Version:     created.Version,
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

	final, err := st.svc.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Version+1, final.Version, "version should increment exactly once")

	if final.Description != "Update A" && final.Description != "Update B" {
		t.Errorf("unexpected final description: %s", final.Description)
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
		permissions []domain.Permission
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
			name:      "revoke outside boundary",
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
			errIs: domain.ErrPermissionEscalation,
		},
		{
			name:      "revoke within boundary",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				_ = enforcer.WriteTx(ctx, txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(casbin.GroupSubject(g.Name), "dev", domain.ObjectNamespace, domain.ActionWrite)
				})
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
			setupGroup: func(_ context.Context, g *domain.Group, _ *casbin.Enforcer, _ storage.TxManager) {
				g.Members = []string{"olduser@example.com"}
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
			created, err := st.svc.Create(ctx, adminAuth(), groupName)
			require.NoError(t, err)

			if tc.setupGroup != nil {
				tc.setupGroup(ctx, created, st.enforcer, st.txm)
				require.NoError(t, st.repo.Update(ctx, created))
			}

			fresh, err := st.svc.Get(ctx, created.ID)
			require.NoError(t, err)

			// Boundary semantics are split between UpdatePermissions and
			// UpdateMembers. We exercise both in sequence; the first one to
			// fail surfaces tc.errIs.
			actor := domain.AuthInfo{Email: tc.principal}

			_, permErr := st.svc.UpdatePermissions(ctx, actor, group.UpdatePermissionsData{
				GroupID:     fresh.ID,
				Permissions: tc.permissions,
			})
			if tc.errIs != nil && permErr != nil {
				require.ErrorIs(t, permErr, tc.errIs)

				return
			}
			require.NoError(t, permErr)

			_, memErr := st.svc.UpdateMembers(ctx, actor, group.UpdateMembersData{
				GroupID: fresh.ID,
				Members: tc.members,
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

				created := &domain.Group{ID: "sys-group-id", Name: "sys-group", System: true, Version: 1}
				require.NoError(t, st.repo.Create(t.Context(), created))

				return group.UpdateData{
					ID:      created.ID,
					Name:    "sys-group",
					Version: created.Version,
				}
			},
			errIs: domain.ErrSystemImmutable,
		},
		{
			name: "version mismatch",
			setup: func(t *testing.T, st testStack) group.UpdateData {
				t.Helper()

				created, err := st.svc.Create(t.Context(), adminAuth(), "ver-group")
				require.NoError(t, err)

				return group.UpdateData{
					ID:      created.ID,
					Name:    "ver-group",
					Version: created.Version + 99,
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
