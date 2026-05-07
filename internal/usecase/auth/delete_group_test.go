package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

// groupGetterDeleter wraps separate getter and deleter mocks into a single type
// that satisfies the anonymous interface{ groupGetter; groupDeleter } used by DeleteGroupUseCase.
type groupGetterDeleter struct {
	*auth_mock.MockgroupGetter
	*auth_mock.MockgroupDeleter
}

func TestDeleteGroupUseCase_Execute(t *testing.T) {
	t.Parallel()

	groupID := "group-123"
	groupName := "devops"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success: group has 2 members and 2 role assignments",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockSync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockDeleter := auth_mock.NewMockgroupDeleter(ctrl)
				repo := &groupGetterDeleter{mockGetter, mockDeleter}

				mockEnforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				mockGetter.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: groupName, Members: []string{"user1@example.com", "user2@example.com"},
				}, nil)

				rules := [][]string{
					{groupName, "role:admin", "ns1"},
					{groupName, "role:writer", "ns2"},
				}
				mockSync.EXPECT().GetRulesForSubject(groupName).Return(rules)

				// Member rules removal
				mockSync.EXPECT().RemoveRoleForUser("user1@example.com", groupName, "ns1")
				mockSync.EXPECT().RemoveRoleForUser("user1@example.com", groupName, "ns2")
				mockSync.EXPECT().RemoveRoleForUser("user2@example.com", groupName, "ns1")
				mockSync.EXPECT().RemoveRoleForUser("user2@example.com", groupName, "ns2")

				// Group rules removal
				mockSync.EXPECT().RemoveRoleForUser(groupName, "role:admin", "ns1")
				mockSync.EXPECT().RemoveRoleForUser(groupName, "role:writer", "ns2")

				mockDeleter.EXPECT().Delete(ctx, groupID).Return(nil)

				return authuc.NewDeleteGroupUseCase(mockEnforcer, mockSync, repo), ctx
			},
		},
		{
			name: "success: group has role assignments but no members",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockSync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockDeleter := auth_mock.NewMockgroupDeleter(ctrl)
				repo := &groupGetterDeleter{mockGetter, mockDeleter}

				mockEnforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				mockGetter.EXPECT().Get(ctx, groupID).Return(&domain.Group{
					ID: groupID, Name: groupName, Members: []string{},
				}, nil)

				rules := [][]string{{groupName, "role:admin", "ns1"}}
				mockSync.EXPECT().GetRulesForSubject(groupName).Return(rules)

				// Group rules removal
				mockSync.EXPECT().RemoveRoleForUser(groupName, "role:admin", "ns1")

				mockDeleter.EXPECT().Delete(ctx, groupID).Return(nil)

				return authuc.NewDeleteGroupUseCase(mockEnforcer, mockSync, repo), ctx
			},
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockEnforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(false, nil)

				return authuc.NewDeleteGroupUseCase(mockEnforcer, nil, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				return authuc.NewDeleteGroupUseCase(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "RemoveRoleForUser returns error -> success (best-effort)",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockSync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockDeleter := auth_mock.NewMockgroupDeleter(ctrl)
				repo := &groupGetterDeleter{mockGetter, mockDeleter}

				mockEnforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				mockGetter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{}}, nil)

				rules := [][]string{{groupName, "role:admin", "ns1"}}
				mockSync.EXPECT().GetRulesForSubject(groupName).Return(rules)
				mockSync.EXPECT().RemoveRoleForUser(groupName, "role:admin", "ns1").Return(assert.AnError)

				mockDeleter.EXPECT().Delete(ctx, groupID).Return(nil)

				return authuc.NewDeleteGroupUseCase(mockEnforcer, mockSync, repo), ctx
			},
		},
		{
			name: "delete fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := auth_mock.NewMockgroupEnforcer(ctrl)
				mockSync := auth_mock.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := auth_mock.NewMockgroupGetter(ctrl)
				mockDeleter := auth_mock.NewMockgroupDeleter(ctrl)
				repo := &groupGetterDeleter{mockGetter, mockDeleter}

				mockEnforcer.EXPECT().Enforce(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				mockGetter.EXPECT().
					Get(ctx, groupID).
					Return(&domain.Group{ID: groupID, Name: groupName, Members: []string{}}, nil)
				mockSync.EXPECT().GetRulesForSubject(groupName).Return(nil)
				mockDeleter.EXPECT().Delete(ctx, groupID).Return(assert.AnError)

				return authuc.NewDeleteGroupUseCase(mockEnforcer, mockSync, repo), ctx
			},
			wantErr: "delete group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Execute(ctx, groupID)

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
