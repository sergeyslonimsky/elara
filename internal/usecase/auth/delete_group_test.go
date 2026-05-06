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

func TestDeleteGroupUseCase_Execute(t *testing.T) {
	t.Parallel()

	groupID := "group-123"

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
				mockGroups := mock_auth.NewMockgroupDeleter(ctrl)

				mockEnforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				mockGroups.EXPECT().Delete(ctx, groupID).Return(nil)

				return authuc.NewDeleteGroupUseCase(mockEnforcer, mockGroups), ctx
			},
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				mockEnforcer := mock_auth.NewMockgroupEnforcer(ctrl)
				mockEnforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(false, nil)

				return authuc.NewDeleteGroupUseCase(mockEnforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				return authuc.NewDeleteGroupUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "delete fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteGroupUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				mockEnforcer := mock_auth.NewMockgroupEnforcer(ctrl)
				mockGroups := mock_auth.NewMockgroupDeleter(ctrl)

				mockEnforcer.EXPECT().
					Enforce("admin@example.com", auth.ObjectAll, "group", auth.ActionWrite).
					Return(true, nil)
				mockGroups.EXPECT().Delete(ctx, groupID).Return(assert.AnError)

				return authuc.NewDeleteGroupUseCase(mockEnforcer, mockGroups), ctx
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
