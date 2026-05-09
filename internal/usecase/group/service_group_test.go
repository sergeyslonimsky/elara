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

func TestService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		mockFunc func(context.Context, *gomock.Controller) (*group.Service, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name:  "success",
			input: "my-group",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().Create(ctx, gomock.Any()).Return(nil)

				return ctx
			}),
		},
		{
			name:  "repo error",
			input: "fail-group",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))

				return ctx
			}),
			wantErr: "create group",
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

			got, err := sut.Create(ctx, tt.input)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.input, got.Name)
			assert.NotEmpty(t, got.ID)
		})
	}
}

func TestService_Get(t *testing.T) {
	t.Parallel()

	groupID := "g1"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Service, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionRead).
					Return(true, nil)
				m.store.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: "test", Members: []string{}}, nil)

				return ctx
			}),
		},
		{
			name: "not found",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().Get(ctx, groupID).Return(nil, domain.NewNotFoundError("group", groupID))

				return ctx
			}),
			errIs:   domain.ErrNotFound,
			wantErr: "get group",
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
					Enforce("user@example.com", auth.ObjectAll, "group", auth.ActionRead).
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

			got, err := sut.Get(ctx, groupID)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, groupID, got.ID)
		})
	}
}

func TestService_Update(t *testing.T) {
	t.Parallel()

	groupID := "g1"
	oldName := "old-name"
	newName := "new-name"

	tests := []struct {
		name     string
		newName  string
		mockFunc func(context.Context, *gomock.Controller) (*group.Service, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name:    "name unchanged -> no Casbin writes",
			newName: oldName,
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: oldName, Members: []string{}}, nil)
				m.store.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				return ctx
			}),
		},
		{
			name:    "name changed, group has 2 role assignments -> Add/Remove called twice",
			newName: newName,
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: oldName, Members: []string{}}, nil)
				m.store.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{
					{oldName, "role:admin", "ns1"},
					{oldName, "role:writer", "ns2"},
				}
				m.syncEnforcer.EXPECT().GetRulesForSubject(oldName).Return(rules)
				m.syncEnforcer.EXPECT().AddRoleForUser(newName, "role:admin", "ns1").Return(nil)
				m.syncEnforcer.EXPECT().RemoveRoleForUser(oldName, "role:admin", "ns1").Return(nil)
				m.syncEnforcer.EXPECT().AddRoleForUser(newName, "role:writer", "ns2").Return(nil)
				m.syncEnforcer.EXPECT().RemoveRoleForUser(oldName, "role:writer", "ns2").Return(nil)

				return ctx
			}),
		},
		{
			name:    "name changed, group has members -> Add/Remove for each member+rule combo",
			newName: newName,
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: oldName, Members: []string{"user1@example.com"},
				}, nil)
				m.store.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				rules := [][]string{{oldName, "role:admin", "ns1"}}
				m.syncEnforcer.EXPECT().GetRulesForSubject(oldName).Return(rules)

				m.syncEnforcer.EXPECT().AddRoleForUser(newName, "role:admin", "ns1").Return(nil)
				m.syncEnforcer.EXPECT().RemoveRoleForUser(oldName, "role:admin", "ns1").Return(nil)

				m.syncEnforcer.EXPECT().AddRoleForUser("user1@example.com", newName, "ns1").Return(nil)
				m.syncEnforcer.EXPECT().RemoveRoleForUser("user1@example.com", oldName, "ns1").Return(nil)

				return ctx
			}),
		},
		{
			name:    "group not found",
			newName: newName,
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().Get(ctx, groupID).Return(nil, domain.NewNotFoundError("group", groupID))

				return ctx
			}),
			wantErr: "get group",
		},
		{
			name:    "update fails -> Casbin untouched",
			newName: newName,
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: oldName, Members: []string{}}, nil)
				m.store.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("update failed"))

				return ctx
			}),
			wantErr: "update group",
		},
		{
			name:    "unauthorized",
			newName: newName,
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				return ctx
			}),
			errIs: domain.ErrUnauthorized,
		},
		{
			name:    "forbidden",
			newName: newName,
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

			got, err := sut.Update(ctx, groupID, tt.newName)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.newName, got.Name)
		})
	}
}

func TestService_Delete(t *testing.T) {
	t.Parallel()

	groupID := "group-123"
	groupName := "devops"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Service, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success: group has 2 members and 2 role assignments",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: groupName, Members: []string{"user1@example.com", "user2@example.com"},
				}, nil)

				rules := [][]string{
					{groupName, "role:admin", "ns1"},
					{groupName, "role:writer", "ns2"},
				}
				m.syncEnforcer.EXPECT().GetRulesForSubject(groupName).Return(rules)

				// Member rules removal
				m.syncEnforcer.EXPECT().RemoveRoleForUser("user1@example.com", groupName, "ns1")
				m.syncEnforcer.EXPECT().RemoveRoleForUser("user1@example.com", groupName, "ns2")
				m.syncEnforcer.EXPECT().RemoveRoleForUser("user2@example.com", groupName, "ns1")
				m.syncEnforcer.EXPECT().RemoveRoleForUser("user2@example.com", groupName, "ns2")

				// Group rules removal
				m.syncEnforcer.EXPECT().RemoveRoleForUser(groupName, "role:admin", "ns1")
				m.syncEnforcer.EXPECT().RemoveRoleForUser(groupName, "role:writer", "ns2")

				m.store.EXPECT().Delete(ctx, groupID).Return(nil)

				return ctx
			}),
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
		{
			name: "unauthorized",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				return ctx
			}),
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "delete fails",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.store.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{}}, nil)
				m.syncEnforcer.EXPECT().GetRulesForSubject(groupName).Return(nil)
				m.store.EXPECT().Delete(ctx, groupID).Return(errors.New("delete failed"))

				return ctx
			}),
			wantErr: "delete group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Delete(ctx, groupID)

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

func TestService_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*group.Service, context.Context)
		errIs    error
		wantErr  string
		want     int
	}{
		{
			name: "success",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionRead).
					Return(true, nil)
				m.store.EXPECT().
					List(ctx).
					Return([]*domain.Group{{ID: "g1"}, {ID: "g2"}}, nil)

				return ctx
			}),
			want: 2,
		},
		{
			name: "unauthorized",
			mockFunc: mockFuncWithContext(func(ctx context.Context, m mocks) context.Context {
				return ctx
			}),
			errIs: domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.List(ctx)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Len(t, got, tt.want)
		})
	}
}
