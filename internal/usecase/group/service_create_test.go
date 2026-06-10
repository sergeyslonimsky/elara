package group_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/storage"
	"github.com/sergeyslonimsky/elara/internal/usecase/group"
)

// TestService_Create covers the orchestrator wiring of Create:
//   - bbolt entity created with the right version counters
//   - p-rules / g-rules applied to the new Casbin subject
//   - manager-group grant (Group:Write group:<new-id>) on each manager
//   - happy-path returns VisibleMembers (filtered) and Permissions
//
// Anti-escalation paths live in TestService_Create_AntiEscalation.
func TestService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// setup may seed extra Casbin state (manager groups, scoped policies)
		// and returns the CreateData to feed Create.
		setup  func(t *testing.T, st testStack) (domain.AuthInfo, group.CreateData)
		assert func(t *testing.T, st testStack, in group.CreateData, got *group.CreateResult)
	}{
		{
			name: "minimal: no members, no perms, no managers",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.CreateData) {
				t.Helper()

				seedAdminWildcard(t, st)

				return adminAuth(), group.CreateData{Name: "minimal"}
			},
			assert: func(t *testing.T, st testStack, in group.CreateData, got *group.CreateResult) {
				t.Helper()

				require.NotNil(t, got.Group)
				assert.NotEmpty(t, got.Group.Name)
				assert.Equal(t, in.Name, got.Group.Name)
				assert.Equal(t, int64(1), got.Group.MetadataVersion)
				assert.Zero(
					t,
					got.Group.MembersVersion,
					"no initial members → MembersVersion stays 0",
				)
				assert.Zero(
					t,
					got.Group.PermissionsVersion,
					"no initial perms → PermissionsVersion stays 0",
				)
				assert.Empty(t, got.VisibleMembers)
				assert.Empty(t, got.Permissions)

				assert.Empty(t, st.enforcer.GetMembersOfGroup(domain.GroupResource(in.Name)),
					"no initial members → no g-rules")
			},
		},
		{
			name: "with initial members only bumps MembersVersion",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.CreateData) {
				t.Helper()

				seedAdminWildcard(t, st)

				return adminAuth(), group.CreateData{
					Name:           "with-members",
					InitialMembers: []string{"user1@example.com", "user2@example.com"},
				}
			},
			assert: func(t *testing.T, st testStack, in group.CreateData, got *group.CreateResult) {
				t.Helper()

				assert.Equal(t, int64(1), got.Group.MetadataVersion)
				assert.Equal(t, int64(1), got.Group.MembersVersion,
					"initial members non-empty → MembersVersion bumped to 1")
				assert.Zero(t, got.Group.PermissionsVersion)

				members := st.enforcer.GetMembersOfGroup(domain.GroupResource(in.Name))
				assert.ElementsMatch(t, []string{"user1@example.com", "user2@example.com"}, members)
				// Wildcard admin → all members visible to admin.
				assert.ElementsMatch(t, in.InitialMembers, got.VisibleMembers)
			},
		},
		{
			name: "with initial permissions only bumps PermissionsVersion",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.CreateData) {
				t.Helper()

				seedAdminWildcard(t, st)

				return adminAuth(), group.CreateData{
					Name: "with-perms",
					InitialPermissions: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "dev"},
					},
				}
			},
			assert: func(t *testing.T, st testStack, in group.CreateData, got *group.CreateResult) {
				t.Helper()

				assert.Equal(t, int64(1), got.Group.MetadataVersion)
				assert.Zero(t, got.Group.MembersVersion)
				assert.Equal(t, int64(1), got.Group.PermissionsVersion,
					"initial perms non-empty → PermissionsVersion bumped to 1")
				assert.ElementsMatch(t, in.InitialPermissions, got.Permissions)

				// p-rule must be attached to the new subject.
				subject := domain.GroupResource(in.Name)
				var hits int
				for _, r := range st.enforcer.GetPolicy() {
					if r[0] == subject {
						hits++
					}
				}
				assert.Equal(t, 1, hits, "exactly one p-rule on the new subject")
			},
		},
		{
			name: "with manager groups grants Group:Write on each manager",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.CreateData) {
				t.Helper()

				seedAdminWildcard(t, st)

				// Seed a manager group "managers" so it has a stable id.
				mg, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "managers"},
				)
				require.NoError(t, err)

				return adminAuth(), group.CreateData{
					Name:                     "child",
					InitialManagerGroupNames: []string{mg.Group.Name},
				}
			},
			assert: func(t *testing.T, st testStack, in group.CreateData, got *group.CreateResult) {
				t.Helper()

				// Manager group "managers" must hold Group:Write group:<new-id>.
				managerSubject := domain.GroupResource("managers")
				wantDomain := domain.GroupResource(got.Group.Name)

				var found bool
				for _, r := range st.enforcer.GetPolicy() {
					if r[0] == managerSubject &&
						r[1] == wantDomain &&
						r[2] == string(domain.ObjectGroup) &&
						r[3] == string(domain.ActionWrite) {
						found = true

						break
					}
				}
				assert.True(t, found, "manager group must hold Group:Write on the new group")
				_ = in
			},
		},
		{
			name: "happy path with members + perms + manager wires everything atomically",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.CreateData) {
				t.Helper()

				seedAdminWildcard(t, st)

				mg, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{
					Name: "mgr",
					// The manager group must itself dominate every initial perm
					// the child receives, or authorizeCreate's cascade check
					// rejects the create.
					InitialPermissions: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
					},
				})
				require.NoError(t, err)

				return adminAuth(), group.CreateData{
					Name:           "full",
					Description:    "full-state",
					InitialMembers: []string{"alice@example.com"},
					InitialPermissions: []domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
					},
					InitialManagerGroupNames: []string{mg.Group.Name},
				}
			},
			assert: func(t *testing.T, st testStack, in group.CreateData, got *group.CreateResult) {
				t.Helper()

				assert.Equal(t, int64(1), got.Group.MetadataVersion)
				assert.Equal(t, int64(1), got.Group.MembersVersion)
				assert.Equal(t, int64(1), got.Group.PermissionsVersion)

				assert.ElementsMatch(t, []string{"alice@example.com"},
					st.enforcer.GetMembersOfGroup(domain.GroupResource(in.Name)))

				// Manager Group:Write grant must exist.
				managerSubject := domain.GroupResource("mgr")
				wantDomain := domain.GroupResource(got.Group.Name)
				var managerGranted, ownPerm bool
				for _, r := range st.enforcer.GetPolicy() {
					if r[0] == managerSubject && r[1] == wantDomain &&
						r[2] == string(domain.ObjectGroup) && r[3] == string(domain.ActionWrite) {
						managerGranted = true
					}
					if r[0] == domain.GroupResource(in.Name) && r[1] == "dev" &&
						r[2] == string(
							domain.ObjectNamespace,
						) && r[3] == string(domain.ActionWrite) {
						ownPerm = true
					}
				}
				assert.True(
					t,
					managerGranted,
					"manager group must hold Group:Write on the new group",
				)
				assert.True(t, ownPerm, "new group must hold its initial permission")

				// And the entity must be retrievable via Get (bbolt commit happened).
				gres, err := st.svc.Get(t.Context(), adminAuth(), got.Group.Name)
				require.NoError(t, err)
				assert.Equal(t, in.Name, gres.Group.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)
			auth, data := tt.setup(t, st)

			got, err := st.svc.Create(t.Context(), auth, data)
			require.NoError(t, err)
			require.NotNil(t, got)
			if tt.assert != nil {
				tt.assert(t, st, data, got)
			}
		})
	}
}

// TestService_Create_AntiEscalation covers the authorizeCreate boundary on
// initial_permissions and the manager-group cascade check.
func TestService_Create_AntiEscalation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, st testStack) (domain.AuthInfo, group.CreateData)
		errIs   error
		wantErr string
	}{
		{
			name: "actor lacks one of initial perms → escalation, no entity created",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.CreateData) {
				t.Helper()

				// devops holds dev:Namespace:Write only — granting prod escalates.
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

				return domain.AuthInfo{UserID: "devops@example.com", Email: "devops@example.com"}, group.CreateData{
					Name: "escalated",
					InitialPermissions: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: "prod",
						},
					},
				}
			},
			errIs: domain.ErrPermissionEscalation,
		},
		{
			name: "manager group lacks an initial perm → cascade escalation",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.CreateData) {
				t.Helper()

				seedAdminWildcard(t, st)

				// Manager group exists but does NOT dominate the initial perm
				// the caller wants to attach to the new group.
				mg, err := st.svc.Create(
					t.Context(),
					adminAuth(),
					group.CreateData{Name: "weak-mgr"},
				)
				require.NoError(t, err)

				return adminAuth(), group.CreateData{
					Name: "child",
					InitialPermissions: []domain.Permission{
						{
							Object: domain.ObjectNamespace,
							Action: domain.ActionWrite,
							Domain: "prod",
						},
					},
					InitialManagerGroupNames: []string{mg.Group.Name},
				}
			},
			// Wrapped message includes manager group name + missing perm fields.
			wantErr: "manager group weak-mgr does not dominate namespace:write on prod",
		},
		{
			name: "actor lacks Group:Write on manager group → forbidden",
			setup: func(t *testing.T, st testStack) (domain.AuthInfo, group.CreateData) {
				t.Helper()

				// Seed a manager group as admin; "stranger" has no perms over it.
				seedAdminWildcard(t, st)
				mg, err := st.svc.Create(t.Context(), adminAuth(), group.CreateData{Name: "mgr"})
				require.NoError(t, err)

				return domain.AuthInfo{UserID: "stranger@example.com", Email: "stranger@example.com"}, group.CreateData{
					Name:                     "outsider-child",
					InitialManagerGroupNames: []string{mg.Group.Name},
				}
			},
			errIs: domain.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := newTestStack(t)
			auth, data := tt.setup(t, st)

			result, err := st.svc.Create(t.Context(), auth, data)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				// And it must still be an escalation sentinel — the cascade
				// fmt.Errorf wraps domain.ErrPermissionEscalation.
				require.ErrorIs(t, err, domain.ErrPermissionEscalation)
			}
			assert.Nil(t, result, "no result returned on failure")

			// Transaction rollback: bbolt must not have a group with this name.
			_, findErr := st.repo.Get(t.Context(), data.Name)
			require.Error(t, findErr, "entity must not be persisted when authorization fails")
			require.ErrorIs(t, findErr, storage.ErrResourceNotFound,
				"Get must report ErrResourceNotFound after rollback, got: %v", findErr)
		})
	}
}

// TestService_Create_NameUniqueness asserts the duplicate-name guard inside
// the same write transaction. Pre-seeds a group then attempts a second create
// with the same name; the second must fail with ErrAlreadyExists and not
// persist a second entity.
func TestService_Create_NameUniqueness(t *testing.T) {
	t.Parallel()

	st := newTestStack(t)
	seedAdminWildcard(t, st)
	ctx := t.Context()

	_, err := st.svc.Create(ctx, adminAuth(), group.CreateData{Name: "dup"})
	require.NoError(t, err)

	_, err = st.svc.Create(ctx, adminAuth(), group.CreateData{Name: "dup"})
	require.ErrorIs(t, err, domain.ErrAlreadyExists)
	require.ErrorContains(t, err, `group "dup"`)

	// Sanity: a different name still works after the duplicate was rejected.
	_, err = st.svc.Create(ctx, adminAuth(), group.CreateData{Name: "unique"})
	require.NoError(t, err)
}
