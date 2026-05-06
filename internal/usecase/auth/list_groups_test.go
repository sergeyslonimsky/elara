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

func TestListGroupsUseCase_Execute(t *testing.T) {
	t.Parallel()

	groups := []*domain.Group{
		{ID: "g1", Name: "Group 1"},
		{ID: "g2", Name: "Group 2"},
	}

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.ListGroupsUseCase, context.Context)
		errIs    error
		wantErr  string
		want     []*domain.Group
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ListGroupsUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				mockEnforcer := mock_auth.NewMockgroupEnforcer(ctrl)
				mockGroups := mock_auth.NewMockgroupLister(ctrl)

				mockEnforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, "group", auth.ActionRead).
					Return(true, nil)
				mockGroups.EXPECT().List(ctx).Return(groups, nil)

				return authuc.NewListGroupsUseCase(mockEnforcer, mockGroups), ctx
			},
			want: groups,
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ListGroupsUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				mockEnforcer := mock_auth.NewMockgroupEnforcer(ctrl)
				mockEnforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, "group", auth.ActionRead).
					Return(false, nil)

				return authuc.NewListGroupsUseCase(mockEnforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ListGroupsUseCase, context.Context) {
				return authuc.NewListGroupsUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "list fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ListGroupsUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				mockEnforcer := mock_auth.NewMockgroupEnforcer(ctrl)
				mockGroups := mock_auth.NewMockgroupLister(ctrl)

				mockEnforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, "group", auth.ActionRead).
					Return(true, nil)
				mockGroups.EXPECT().List(ctx).Return(nil, assert.AnError)

				return authuc.NewListGroupsUseCase(mockEnforcer, mockGroups), ctx
			},
			wantErr: "list groups",
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
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
