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

func TestCreateUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *domain.Namespace
		mockFunc func(context.Context, *gomock.Controller) (*namespace.CreateUseCase, context.Context)
		errIs    error
		wantErr  string
		want     *domain.Namespace
	}{
		{
			name: "success",
			input: &domain.Namespace{
				Name: "prod",
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.CreateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				enforcer := mock_namespace.NewMockcreateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("admin@example.com", "*", "namespace", "write").Return(true, nil)

				namespaces := mock_namespace.NewMocknsCreator(ctrl)
				namespaces.EXPECT().Create(ctx, gomock.Any()).Return(nil)

				getter := mock_namespace.NewMocknsGetterForCreate(ctrl)
				getter.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				return namespace.NewCreateUseCase(enforcer, namespaces, getter), ctx
			},
			want: &domain.Namespace{Name: "prod"},
		},
		{
			name:  "unauthorized",
			input: &domain.Namespace{Name: "prod"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.CreateUseCase, context.Context) {
				return namespace.NewCreateUseCase(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:  "forbidden",
			input: &domain.Namespace{Name: "prod"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.CreateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_namespace.NewMockcreateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "*", "namespace", "write").Return(false, nil)
				return namespace.NewCreateUseCase(enforcer, nil, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:  "validation error",
			input: &domain.Namespace{Name: ""},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.CreateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				enforcer := mock_namespace.NewMockcreateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("admin@example.com", "*", "namespace", "write").Return(true, nil)
				return namespace.NewCreateUseCase(enforcer, nil, nil), ctx
			},
			wantErr: "validate namespace",
		},
		{
			name:  "create error",
			input: &domain.Namespace{Name: "prod"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.CreateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				enforcer := mock_namespace.NewMockcreateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("admin@example.com", "*", "namespace", "write").Return(true, nil)

				namespaces := mock_namespace.NewMocknsCreator(ctrl)
				namespaces.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))

				return namespace.NewCreateUseCase(enforcer, namespaces, nil), ctx
			},
			wantErr: "create namespace: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.input)

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
