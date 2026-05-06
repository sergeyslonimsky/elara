package namespace_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	mock_namespace "github.com/sergeyslonimsky/elara/internal/usecase/namespace/mocks"
)

func TestGetUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nsName   string
		mockFunc func(context.Context, *gomock.Controller) (*namespace.GetUseCase, context.Context)
		errIs    error
		wantErr  string
		want     *domain.Namespace
	}{
		{
			name:   "success",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_namespace.NewMockgetEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "read").Return(true, nil)

				namespaces := mock_namespace.NewMocknsGetter(ctrl)
				namespaces.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				counter := mock_namespace.NewMockgetConfigCounter(ctrl)
				counter.EXPECT().CountConfigs(ctx, "prod").Return(5, nil)

				return namespace.NewGetUseCase(enforcer, namespaces, counter), ctx
			},
			want: &domain.Namespace{Name: "prod", ConfigCount: 5},
		},
		{
			name:   "unauthorized",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.GetUseCase, context.Context) {
				return namespace.NewGetUseCase(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:   "forbidden",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_namespace.NewMockgetEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "read").Return(false, nil)
				return namespace.NewGetUseCase(enforcer, nil, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:   "namespace not found",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_namespace.NewMockgetEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "read").Return(true, nil)

				namespaces := mock_namespace.NewMocknsGetter(ctrl)
				namespaces.EXPECT().Get(ctx, "prod").Return(nil, domain.ErrNotFound)

				return namespace.NewGetUseCase(enforcer, namespaces, nil), ctx
			},
			wantErr: "get namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.nsName)

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
