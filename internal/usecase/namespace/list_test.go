package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	mock_namespace "github.com/sergeyslonimsky/elara/internal/usecase/namespace/mocks"
)

func TestListUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   namespace.NSListParams
		mockFunc func(context.Context, *gomock.Controller) (*namespace.ListUseCase, context.Context)
		errIs    error
		wantErr  string
		want     *namespace.NSListResult
	}{
		{
			name: "success",
			params: namespace.NSListParams{
				Limit: 10,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.ListUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_namespace.NewMocklistEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", auth.ObjectNamespace, auth.ActionRead).Return(true, nil)
				enforcer.EXPECT().Enforce("user@example.com", "dev", auth.ObjectNamespace, auth.ActionRead).Return(true, nil)

				namespaces := mock_namespace.NewMocknsLister(ctrl)
				list := []*domain.Namespace{
					{Name: "prod"},
					{Name: "dev"},
				}
				namespaces.EXPECT().List(ctx).Return(list, nil)

				counter := mock_namespace.NewMocklistConfigCounter(ctrl)
				counter.EXPECT().CountConfigs(ctx, "dev").Return(5, nil)
				counter.EXPECT().CountConfigs(ctx, "prod").Return(10, nil)

				return namespace.NewListUseCase(enforcer, namespaces, counter), ctx
			},
			want: &namespace.NSListResult{
				Namespaces: []*domain.Namespace{
					{Name: "dev", ConfigCount: 5},
					{Name: "prod", ConfigCount: 10},
				},
				Total: 2,
				Limit: 10,
			},
		},
		{
			name:   "filter by permission",
			params: namespace.NSListParams{Limit: 10},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.ListUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_namespace.NewMocklistEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", auth.ObjectNamespace, auth.ActionRead).Return(true, nil)
				enforcer.EXPECT().Enforce("user@example.com", "dev", auth.ObjectNamespace, auth.ActionRead).Return(false, nil)

				namespaces := mock_namespace.NewMocknsLister(ctrl)
				list := []*domain.Namespace{
					{Name: "prod"},
					{Name: "dev"},
				}
				namespaces.EXPECT().List(ctx).Return(list, nil)

				counter := mock_namespace.NewMocklistConfigCounter(ctrl)
				counter.EXPECT().CountConfigs(ctx, "prod").Return(10, nil)

				return namespace.NewListUseCase(enforcer, namespaces, counter), ctx
			},
			want: &namespace.NSListResult{
				Namespaces: []*domain.Namespace{
					{Name: "prod", ConfigCount: 10},
				},
				Total: 1,
				Limit: 10,
			},
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.ListUseCase, context.Context) {
				return namespace.NewListUseCase(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "list error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.ListUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				namespaces := mock_namespace.NewMocknsLister(ctrl)
				namespaces.EXPECT().List(ctx).Return(nil, errors.New("db error"))
				return namespace.NewListUseCase(nil, namespaces, nil), ctx
			},
			wantErr: "list namespaces: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.params)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.Total, got.Total)
			require.Len(t, got.Namespaces, len(tt.want.Namespaces))
			for i := range got.Namespaces {
				assert.Equal(t, tt.want.Namespaces[i].Name, got.Namespaces[i].Name)
				assert.Equal(t, tt.want.Namespaces[i].ConfigCount, got.Namespaces[i].ConfigCount)
			}
		})
	}
}
