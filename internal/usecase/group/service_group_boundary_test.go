package group_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

func contextWithClaims(ctx context.Context, email string) context.Context {
	return auth.WithClaims(ctx, &auth.Claims{Email: email})
}

func TestService_Update_Boundary(t *testing.T) {
	t.Parallel()

	// This outer service is just to seed the initial roles for the "global" DB, but actually we are
	// seeding a fresh DB per test case below, so this setup is just for validation.
	// We'll skip outer setup to avoid unused variables, and do it strictly per-test.

	type testCase struct {
		name        string
		principal   string
		setupGroup  func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) // customize initial group state before test
		permissions []domain.Permission
		members     []string
		wantErr     error
	}

	tests := []testCase{
		// T4.2 - permissions boundary
		{
			name:      "grant within boundary",
			principal: "devops@example.com",
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
			},
			members: nil,
			wantErr: nil, // success
		},
		{
			name:      "grant outside boundary",
			principal: "devops@example.com",
			permissions: []domain.Permission{
				{
					Object: domain.ObjectNamespace,
					Action: domain.ActionWrite,
					Domain: "prod",
				}, // devops doesn't have this
			},
			members: nil,
			wantErr: domain.ErrPermissionEscalation,
		},
		{
			name:      "revoke outside boundary",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				// The group already has prod write
				_ = enforcer.WriteTx(ctx, txm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						domain.GroupSubject(g.Name),
						"prod",
						domain.ObjectNamespace,
						domain.ActionWrite,
					)
				})
			},
			permissions: nil, // trying to remove the existing prod write
			members:     nil,
			wantErr:     domain.ErrPermissionEscalation,
		},
		{
			name:      "revoke within boundary",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				// The group already has dev write
				_ = enforcer.WriteTx(ctx, txm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(domain.GroupSubject(g.Name), "dev", domain.ObjectNamespace, domain.ActionWrite)
				})
			},
			permissions: nil, // trying to remove dev write, which devops has
			members:     nil,
			wantErr:     nil, // success
		},
		{
			name:      "superadmin wildcard",
			principal: "admin@example.com", // has (*,*,*)
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"},
			},
			members: nil,
			wantErr: nil, // success
		},

		// T4.3 - union rule for members
		{
			name:      "member-add to group with outside-boundary perm",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				_ = enforcer.WriteTx(ctx, txm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						domain.GroupSubject(g.Name),
						"prod",
						domain.ObjectNamespace,
						domain.ActionWrite,
					)
				})
			},
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"}, // unchanged perm
			},
			members: []string{"newuser@example.com"},
			wantErr: domain.ErrPermissionEscalation,
		},
		{
			name:      "member-remove from group with outside-boundary perm",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				g.Members = []string{"olduser@example.com"} // bypass standard group creation logic for speed
				_ = enforcer.WriteTx(ctx, txm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						domain.GroupSubject(g.Name),
						"prod",
						domain.ObjectNamespace,
						domain.ActionWrite,
					)
				})
			},
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"}, // unchanged perm
			},
			members: nil, // remove olduser
			wantErr: domain.ErrPermissionEscalation,
		},
		{
			name:      "member change with all perms in boundary",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				g.Members = []string{"olduser@example.com"}
				_ = enforcer.WriteTx(ctx, txm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(domain.GroupSubject(g.Name), "dev", domain.ObjectNamespace, domain.ActionWrite)
				})
			},
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"},
			},
			members: []string{"newuser@example.com"}, // replace olduser with newuser
			wantErr: nil,
		},
		{
			name:      "passthrough: simultaneous perm change + member change, all in boundary",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				g.Members = []string{"olduser@example.com"}
				// initially empty perms
			},
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "dev"}, // adding dev perm
			},
			members: []string{"newuser@example.com"}, // adding member
			wantErr: nil,
		},
		{
			name:      "passthrough loophole: member-add при unchanged outside-boundary perm",
			principal: "devops@example.com",
			setupGroup: func(ctx context.Context, g *domain.Group, enforcer *casbin.Enforcer, txm storage.TxManager) {
				_ = enforcer.WriteTx(ctx, txm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
					return txe.AddPolicy(
						domain.GroupSubject(g.Name),
						"prod",
						domain.ObjectNamespace,
						domain.ActionWrite,
					)
				})
			},
			permissions: []domain.Permission{
				{Object: domain.ObjectNamespace, Action: domain.ActionWrite, Domain: "prod"}, // unchanged
			},
			members: []string{"hacker@example.com"},
			wantErr: domain.ErrPermissionEscalation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Boot a new stack per test
			subSut, subBStore, subEnforcer, subGroupRepo := newTestService(t)
			subTxm := bbolt.NewTxManager(subBStore.DB())

			subCtx := t.Context()
			_ = subEnforcer.WriteTx(subCtx, subTxm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
				_ = txe.AddPolicy("admin@example.com", domain.DomainAll, domain.ObjectAll, domain.ActionAll)
				_ = txe.AddPolicy("devops@example.com", "dev", domain.ObjectNamespace, domain.ActionWrite)

				return nil
			})

			groupName := "group-" + tc.name
			created, err := subSut.Create(subCtx, groupName)
			require.NoError(t, err)

			if tc.setupGroup != nil {
				tc.setupGroup(subCtx, created, subEnforcer, subTxm)
				// If setupGroup modified fields like Members, we need to save it via store.
				_ = subGroupRepo.Update(subCtx, created)
			}

			// We need a fresh copy after setup just in case.
			fresh, err := subSut.Get(subCtx, created.ID)
			require.NoError(t, err)

			// Perform update
			reqCtx := contextWithClaims(subCtx, tc.principal)
			updated, err := subSut.Update(
				reqCtx,
				fresh.ID,
				fresh.Name,
				fresh.Description,
				tc.permissions,
				tc.members,
				fresh.Version,
			)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, updated)
			}
		})
	}
}

func TestService_Update_ImmutabilityAndVersion(t *testing.T) {
	t.Parallel()

	sut, _, _, store := newTestService(t)
	ctx := contextWithAdmin(t.Context())

	t.Run("system group", func(t *testing.T) {
		t.Parallel()

		created := &domain.Group{ID: "sys-group-id", Name: "sys-group", System: true, Version: 1}
		err := store.Create(ctx, created)
		require.NoError(t, err)

		_, err = sut.Update(ctx, created.ID, "sys-group", "", nil, nil, created.Version)
		require.ErrorIs(t, err, domain.ErrSystemImmutable)
	})

	t.Run("version mismatch", func(t *testing.T) {
		t.Parallel()

		created, err := sut.Create(ctx, "ver-group")
		require.NoError(t, err)

		_, err = sut.Update(ctx, created.ID, "ver-group", "", nil, nil, created.Version+99)
		require.ErrorIs(t, err, domain.ErrVersionConflict)
	})
}
