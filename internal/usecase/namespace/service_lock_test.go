package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func TestService_Lock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nsName   string
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
	}{
		{
			name:   "success",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", "prod", auth.ObjectNamespace, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().LockNamespace(ctx, "prod").Return(nil)
				m.notifier.EXPECT().NotifyNamespaceLocked(ctx, "prod")

				return ctx
			},
		},
		{
			name:   "unauthorized",
			nsName: "prod",
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:   "forbidden",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectNamespace, auth.ActionWrite).
					Return(false, nil)

				return ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:   "store error",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", "prod", auth.ObjectNamespace, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().LockNamespace(ctx, "prod").Return(errors.New("db error"))

				return ctx
			},
			wantErr: "lock namespace: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			err := svc.Lock(ctx, tt.nsName)

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

func TestService_Unlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nsName   string
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
	}{
		{
			name:   "success",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", "prod", auth.ObjectNamespace, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().UnlockNamespace(ctx, "prod").Return(nil)
				m.notifier.EXPECT().NotifyNamespaceUnlocked(ctx, "prod")

				return ctx
			},
		},
		{
			name:   "unauthorized",
			nsName: "prod",
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:   "forbidden",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectNamespace, auth.ActionWrite).
					Return(false, nil)

				return ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:   "store error",
			nsName: "prod",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "admin@example.com"})
				m.enforcer.EXPECT().
					Enforce("admin@example.com", "prod", auth.ObjectNamespace, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().UnlockNamespace(ctx, "prod").Return(errors.New("db error"))

				return ctx
			},
			wantErr: "unlock namespace: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			err := svc.Unlock(ctx, tt.nsName)

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
