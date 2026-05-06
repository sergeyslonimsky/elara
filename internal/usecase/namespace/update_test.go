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

func TestUpdateUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		nsName      string
		description string
		mockFunc    func(context.Context, *gomock.Controller) (*namespace.UpdateUseCase, context.Context)
		errIs       error
		wantErr     string
		want        *domain.Namespace
	}{
		{
			name:        "success",
			nsName:      "prod",
			description: "Production",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.UpdateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				enforcer := mock_namespace.NewMockupdateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("admin@example.com", "prod", "namespace", "write").Return(true, nil)

				namespaces := mock_namespace.NewMocknsUpdater(ctrl)
				namespaces.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				getter := mock_namespace.NewMocknsGetterForUpdate(ctrl)
				getter.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod", Description: "Production"}, nil)

				counter := mock_namespace.NewMockupdateConfigCounter(ctrl)
				counter.EXPECT().CountConfigs(ctx, "prod").Return(10, nil)

				return namespace.NewUpdateUseCase(enforcer, namespaces, getter, counter), ctx
			},
			want: &domain.Namespace{Name: "prod", Description: "Production", ConfigCount: 10},
		},
		{
			name:   "unauthorized",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.UpdateUseCase, context.Context) {
				return namespace.NewUpdateUseCase(nil, nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:   "forbidden",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.UpdateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_namespace.NewMockupdateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "write").Return(false, nil)

				return namespace.NewUpdateUseCase(enforcer, nil, nil, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:   "update error",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.UpdateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				enforcer := mock_namespace.NewMockupdateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("admin@example.com", "prod", "namespace", "write").Return(true, nil)

				namespaces := mock_namespace.NewMocknsUpdater(ctrl)
				namespaces.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db error"))

				return namespace.NewUpdateUseCase(enforcer, namespaces, nil, nil), ctx
			},
			wantErr: "update namespace: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.nsName, tt.description)

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
