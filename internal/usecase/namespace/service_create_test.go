package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func TestService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *domain.Namespace
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
		want     *domain.Namespace
	}{
		{
			name: "success",
			input: &domain.Namespace{
				Name: "prod",
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})

				m.enforcer.EXPECT().Enforce("admin@example.com", "*", "namespace", "write").Return(true, nil)
				m.store.EXPECT().Create(ctx, gomock.Any()).Return(nil)
				m.store.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				return ctx
			},
			want: &domain.Namespace{Name: "prod"},
		},
		{
			name:  "unauthorized",
			input: &domain.Namespace{Name: "prod"},
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:  "forbidden",
			input: &domain.Namespace{Name: "prod"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().Enforce("user@example.com", "*", "namespace", "write").Return(false, nil)

				return ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:  "validation error",
			input: &domain.Namespace{Name: ""},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce("admin@example.com", "*", "namespace", "write").Return(true, nil)

				return ctx
			},
			wantErr: "validate namespace",
		},
		{
			name:  "create error",
			input: &domain.Namespace{Name: "prod"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().Enforce("admin@example.com", "*", "namespace", "write").Return(true, nil)
				m.store.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))

				return ctx
			},
			wantErr: "create namespace: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.Create(ctx, tt.input)

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
