package group_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/group"
)

func TestService_AddMember(t *testing.T) {
	t.Parallel()

	groupID := "g1"
	groupName := "devops"
	email := "user@example.com"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Service, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "group has 2 role assignments -> AddRoleForUser called twice",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.enforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{}}, nil)
				m.store.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{
					{groupName, "role:admin", "ns1"},
					{groupName, "role:writer", "ns2"},
				}
				m.syncEnforcer.EXPECT().GetRulesForSubject(groupName).Return(rules)
				m.syncEnforcer.EXPECT().AddRoleForUser(email, groupName, "ns1").Return(nil)
				m.syncEnforcer.EXPECT().AddRoleForUser(email, groupName, "ns2").Return(nil)

				return ctx
			}),
		},
		{
			name: "group not found",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().Get(ctx, groupID).Return(nil, domain.NewNotFoundError("group", groupID))

				return ctx
			}),
			wantErr: "get group",
		},
		{
			name: "duplicate member returns error",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: groupName, Members: []string{email},
				}, nil)

				return ctx
			}),
			wantErr: "add member",
		},
		{
			name: "update fails",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: groupName, Members: []string{},
				}, nil)
				m.store.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("update failed"))

				return ctx
			}),
			wantErr: "update group",
		},
		{
			name: "Casbin sync error -> returns error",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.enforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{}}, nil)
				m.store.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{{groupName, "role:admin", "ns1"}}
				m.syncEnforcer.EXPECT().GetRulesForSubject(groupName).Return(rules)
				m.syncEnforcer.EXPECT().AddRoleForUser(email, groupName, "ns1").Return(errors.New("sync failed"))

				return ctx
			}),
			wantErr: "sync member role",
		},
		{
			name: "unauthorized",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				return ctx
			}),
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "forbidden",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(false, nil)

				return ctx
			}),
			errIs: domain.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.AddMember(ctx, groupID, email)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Contains(t, got.Members, email)
		})
	}
}

func TestService_RemoveMember(t *testing.T) {
	t.Parallel()

	groupID := "g1"
	groupName := "devops"
	email := "user@example.com"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Service, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "group has 2 role assignments -> RemoveRoleForUser called twice",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.enforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{email}}, nil)
				m.store.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{
					{groupName, "role:admin", "ns1"},
					{groupName, "role:writer", "ns2"},
				}
				m.syncEnforcer.EXPECT().GetRulesForSubject(groupName).Return(rules)
				m.syncEnforcer.EXPECT().RemoveRoleForUser(email, groupName, "ns1").Return(nil)
				m.syncEnforcer.EXPECT().RemoveRoleForUser(email, groupName, "ns2").Return(nil)

				return ctx
			}),
		},
		{
			name: "member not found returns error",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: groupName, Members: []string{},
				}, nil)

				return ctx
			}),
			wantErr: "remove member",
		},
		{
			name: "Casbin sync error -> returns error",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.enforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{email}}, nil)
				m.store.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{{groupName, "role:admin", "ns1"}}
				m.syncEnforcer.EXPECT().GetRulesForSubject(groupName).Return(rules)
				m.syncEnforcer.EXPECT().RemoveRoleForUser(email, groupName, "ns1").Return(errors.New("sync failed"))

				return ctx
			}),
			wantErr: "sync remove member role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.RemoveMember(ctx, groupID, email)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.NotContains(t, got.Members, email)
		})
	}
}
