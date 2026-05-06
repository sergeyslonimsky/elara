package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/namespace"
	mock_namespace "github.com/sergeyslonimsky/elara/internal/usecase/namespace/mocks"
)

func TestDeleteUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nsName   string
		mockFunc func(context.Context, *gomock.Controller) (*namespace.DeleteUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name:   "success",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.DeleteUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				enforcer := mock_namespace.NewMockdeleteEnforcer(ctrl)
				enforcer.EXPECT().Enforce("admin@example.com", "prod", "namespace", "write").Return(true, nil)

				counter := mock_namespace.NewMocknsConfigCounter(ctrl)
				counter.EXPECT().CountConfigs(ctx, "prod").Return(0, nil)

				namespaces := mock_namespace.NewMocknsDeleter(ctrl)
				namespaces.EXPECT().Delete(ctx, "prod").Return(nil)

				return namespace.NewDeleteUseCase(enforcer, namespaces, counter), ctx
			},
		},
		{
			name:   "unauthorized",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.DeleteUseCase, context.Context) {
				return namespace.NewDeleteUseCase(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:   "forbidden",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.DeleteUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_namespace.NewMockdeleteEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "namespace", "write").Return(false, nil)
				return namespace.NewDeleteUseCase(enforcer, nil, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:   "not empty",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.DeleteUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				enforcer := mock_namespace.NewMockdeleteEnforcer(ctrl)
				enforcer.EXPECT().Enforce("admin@example.com", "prod", "namespace", "write").Return(true, nil)

				counter := mock_namespace.NewMocknsConfigCounter(ctrl)
				counter.EXPECT().CountConfigs(ctx, "prod").Return(5, nil)

				return namespace.NewDeleteUseCase(enforcer, nil, counter), ctx
			},
			wantErr: `namespace "prod" contains 5 config(s)`,
		},
		{
			name:   "delete error",
			nsName: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*namespace.DeleteUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				enforcer := mock_namespace.NewMockdeleteEnforcer(ctrl)
				enforcer.EXPECT().Enforce("admin@example.com", "prod", "namespace", "write").Return(true, nil)

				counter := mock_namespace.NewMocknsConfigCounter(ctrl)
				counter.EXPECT().CountConfigs(ctx, "prod").Return(0, nil)

				namespaces := mock_namespace.NewMocknsDeleter(ctrl)
				namespaces.EXPECT().Delete(ctx, "prod").Return(errors.New("db error"))

				return namespace.NewDeleteUseCase(enforcer, namespaces, counter), ctx
			},
			wantErr: "delete namespace: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Execute(ctx, tt.nsName)

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
