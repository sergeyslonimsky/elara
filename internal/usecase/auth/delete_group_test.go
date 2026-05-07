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
	mock_auth "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

// groupGetterDeleter wraps separate getter and deleter mocks into a single type
// that satisfies the anonymous interface{ groupGetter; groupDeleter } used by DeleteGroupUseCase.
type groupGetterDeleter struct {
	*mock_auth.MockgroupGetter
	*mock_auth.MockgroupDeleter
}

func TestDeleteGroupUseCase_Execute(t *testing.T) {
	t.Parallel()

	groupID := "group-123"
	existingGroup := &domain.Group{ID: groupID, Name: "devops", Members: []string{}}

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := mock_auth.NewMockgroupEnforcer(ctrl)
				mockSync := mock_auth.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := mock_auth.NewMockgroupGetter(ctrl)
				mockDeleter := mock_auth.NewMockgroupDeleter(ctrl)
				repo := &groupGetterDeleter{mockGetter, mockDeleter}

				mockEnforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				mockGetter.EXPECT().Get(ctx, groupID).Return(existingGroup, nil)
				mockSync.EXPECT().GetRulesForSubject("devops").Return(nil)
				mockDeleter.EXPECT().Delete(ctx, groupID).Return(nil)

				return authuc.NewDeleteGroupUseCase(mockEnforcer, mockSync, repo), ctx
			},
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				mockEnforcer := mock_auth.NewMockgroupEnforcer(ctrl)
				mockSync := mock_auth.NewMockgroupSyncEnforcer(ctrl)
				mockEnforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(false, nil)

				return authuc.NewDeleteGroupUseCase(mockEnforcer, mockSync, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				return authuc.NewDeleteGroupUseCase(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "delete fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := mock_auth.NewMockgroupEnforcer(ctrl)
				mockSync := mock_auth.NewMockgroupSyncEnforcer(ctrl)
				mockGetter := mock_auth.NewMockgroupGetter(ctrl)
				mockDeleter := mock_auth.NewMockgroupDeleter(ctrl)
				repo := &groupGetterDeleter{mockGetter, mockDeleter}

				mockEnforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				mockGetter.EXPECT().Get(ctx, groupID).Return(existingGroup, nil)
				mockSync.EXPECT().GetRulesForSubject("devops").Return(nil)
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
