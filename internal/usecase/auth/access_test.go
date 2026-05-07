package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestAssignRoleUseCase_Execute(t *testing.T) {
	t.Parallel()

	subject := "devops"
	dom := "prod"
	role := "role:admin"
	adminEmail := "admin@example.com"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.AssignRoleUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "assign: subject is a group with 2 members -> members synced",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AssignRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				groups := auth_mock.NewMockgroupByNameFinder(ctrl)

				enforcer.EXPECT().Enforce(adminEmail, "*", "policy", "write").Return(true, nil)
				enforcer.EXPECT().AddRoleForUser(subject, role, dom).Return(nil)

				groups.EXPECT().FindByName(ctx, subject).Return(&domain.Group{
					Name: subject, Members: []string{"user1@example.com", "user2@example.com"},
				}, nil)

				enforcer.EXPECT().AddRoleForUser("user1@example.com", subject, dom).Return(nil)
				enforcer.EXPECT().AddRoleForUser("user2@example.com", subject, dom).Return(nil)

				return authuc.NewAssignRoleUseCase(enforcer, groups), ctx
			},
		},
		{
			name: "assign: subject is a group with no members -> no member calls",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AssignRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				groups := auth_mock.NewMockgroupByNameFinder(ctrl)

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				enforcer.EXPECT().AddRoleForUser(subject, role, dom).Return(nil)

				groups.EXPECT().FindByName(ctx, subject).Return(&domain.Group{
					Name: subject, Members: []string{},
				}, nil)

				return authuc.NewAssignRoleUseCase(enforcer, groups), ctx
			},
		},
		{
			name: "assign: FindByName returns ErrNotFound -> success (subject is a user)",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AssignRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				groups := auth_mock.NewMockgroupByNameFinder(ctrl)

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				enforcer.EXPECT().AddRoleForUser(subject, role, dom).Return(nil)

				groups.EXPECT().FindByName(ctx, subject).Return(nil, domain.ErrNotFound)

				return authuc.NewAssignRoleUseCase(enforcer, groups), ctx
			},
		},
		{
			name: "assign: FindByName returns unexpected error -> returns error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AssignRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				groups := auth_mock.NewMockgroupByNameFinder(ctrl)

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				enforcer.EXPECT().AddRoleForUser(subject, role, dom).Return(nil)

				groups.EXPECT().FindByName(ctx, subject).Return(nil, errors.New("db error"))

				return authuc.NewAssignRoleUseCase(enforcer, groups), ctx
			},
			wantErr: "find group by name",
		},
		{
			name: "enforce error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AssignRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce(adminEmail, "*", "policy", "write").
					Return(false, errors.New("enforce failed"))

				return authuc.NewAssignRoleUseCase(enforcer, nil), ctx
			},
			wantErr: "enforce",
		},
		{
			name: "sync group member fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AssignRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				groups := auth_mock.NewMockgroupByNameFinder(ctrl)

				enforcer.EXPECT().Enforce(adminEmail, "*", "policy", "write").Return(true, nil)
				enforcer.EXPECT().AddRoleForUser(subject, role, dom).Return(nil)

				groups.EXPECT().FindByName(ctx, subject).Return(&domain.Group{
					Name: subject, Members: []string{"member@example.com"},
				}, nil)
				enforcer.EXPECT().AddRoleForUser("member@example.com", subject, dom).Return(errors.New("casbin error"))

				return authuc.NewAssignRoleUseCase(enforcer, groups), ctx
			},
			wantErr: "sync group member",
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*authuc.AssignRoleUseCase, context.Context) {
				return authuc.NewAssignRoleUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.AssignRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				enforcer.EXPECT().Enforce(adminEmail, "*", "policy", "write").Return(false, nil)

				return authuc.NewAssignRoleUseCase(enforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Execute(ctx, subject, dom, role)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRevokeRoleUseCase_Execute(t *testing.T) {
	t.Parallel()

	subject := "devops"
	dom := "prod"
	role := "role:admin"
	adminEmail := "admin@example.com"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.RevokeRoleUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "revoke: subject is a group with 2 members -> members synced",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RevokeRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				groups := auth_mock.NewMockgroupByNameFinder(ctrl)

				enforcer.EXPECT().Enforce(adminEmail, "*", "policy", "write").Return(true, nil)
				enforcer.EXPECT().RemoveRoleForUser(subject, role, dom).Return(nil)

				groups.EXPECT().FindByName(ctx, subject).Return(&domain.Group{
					Name: subject, Members: []string{"user1@example.com", "user2@example.com"},
				}, nil)

				enforcer.EXPECT().RemoveRoleForUser("user1@example.com", subject, dom).Return(nil)
				enforcer.EXPECT().RemoveRoleForUser("user2@example.com", subject, dom).Return(nil)

				return authuc.NewRevokeRoleUseCase(enforcer, groups), ctx
			},
		},
		{
			name: "revoke: subject is a group with no members -> no member calls",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RevokeRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				groups := auth_mock.NewMockgroupByNameFinder(ctrl)

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				enforcer.EXPECT().RemoveRoleForUser(subject, role, dom).Return(nil)

				groups.EXPECT().FindByName(ctx, subject).Return(&domain.Group{
					Name: subject, Members: []string{},
				}, nil)

				return authuc.NewRevokeRoleUseCase(enforcer, groups), ctx
			},
		},
		{
			name: "revoke: FindByName returns unexpected error -> returns error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RevokeRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				groups := auth_mock.NewMockgroupByNameFinder(ctrl)

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				enforcer.EXPECT().RemoveRoleForUser(subject, role, dom).Return(nil)

				groups.EXPECT().FindByName(ctx, subject).Return(nil, errors.New("db error"))

				return authuc.NewRevokeRoleUseCase(enforcer, groups), ctx
			},
			wantErr: "find group by name",
		},
		{
			name: "enforce error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RevokeRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce(adminEmail, "*", "policy", "write").
					Return(false, errors.New("enforce failed"))

				return authuc.NewRevokeRoleUseCase(enforcer, nil), ctx
			},
			wantErr: "enforce",
		},
		{
			name: "sync revoke group member fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RevokeRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				groups := auth_mock.NewMockgroupByNameFinder(ctrl)

				enforcer.EXPECT().Enforce(adminEmail, "*", "policy", "write").Return(true, nil)
				enforcer.EXPECT().RemoveRoleForUser(subject, role, dom).Return(nil)

				groups.EXPECT().FindByName(ctx, subject).Return(&domain.Group{
					Name: subject, Members: []string{"member@example.com"},
				}, nil)
				enforcer.EXPECT().
					RemoveRoleForUser("member@example.com", subject, dom).
					Return(errors.New("casbin error"))

				return authuc.NewRevokeRoleUseCase(enforcer, groups), ctx
			},
			wantErr: "sync revoke group member",
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*authuc.RevokeRoleUseCase, context.Context) {
				return authuc.NewRevokeRoleUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.RevokeRoleUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				enforcer.EXPECT().Enforce(adminEmail, "*", "policy", "write").Return(false, nil)

				return authuc.NewRevokeRoleUseCase(enforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Execute(ctx, subject, dom, role)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestListPoliciesUseCase_Execute(t *testing.T) {
	t.Parallel()

	adminEmail := "admin@example.com"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.ListPoliciesUseCase, context.Context)
		wantLen  int
		errIs    error
	}{
		{
			name: "filters out membership records, keeps known roles",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ListPoliciesUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)

				enforcer.EXPECT().Enforce(adminEmail, "*", "policy", "read").Return(true, nil)
				rules := [][]string{
					{"user@example.com", "admin", "*"},
					{"devops", "writer", "prod"},
					{"user@example.com", "devops", "prod"}, // membership record — filtered out
				}
				enforcer.EXPECT().GetGroupingPolicy().Return(rules)

				return authuc.NewListPoliciesUseCase(enforcer), ctx
			},
			wantLen: 2,
		},
		{
			name: "all rules are membership records -> empty result",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ListPoliciesUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				enforcer.EXPECT().GetGroupingPolicy().Return([][]string{
					{"user1@example.com", "devops", "prod"},
					{"user2@example.com", "devops", "prod"},
				})

				return authuc.NewListPoliciesUseCase(enforcer), ctx
			},
			wantLen: 0,
		},
		{
			name: "empty rules -> empty result",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ListPoliciesUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)

				enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				enforcer.EXPECT().GetGroupingPolicy().Return(nil)

				return authuc.NewListPoliciesUseCase(enforcer), ctx
			},
			wantLen: 0,
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*authuc.ListPoliciesUseCase, context.Context) {
				return authuc.NewListPoliciesUseCase(nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ListPoliciesUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
				enforcer.EXPECT().Enforce(adminEmail, "*", "policy", "read").Return(false, nil)

				return authuc.NewListPoliciesUseCase(enforcer), ctx
			},
			errIs: domain.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			require.NoError(t, err)
			assert.Len(t, got, tt.wantLen)
		})
	}
}
