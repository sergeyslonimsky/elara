package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Authorization is enforced at the handler boundary; these tests cover only
// the business behaviour of Lock and Unlock.

func TestService_Lock(t *testing.T) {
	t.Parallel()

	const name = "prod"

	tests := []struct {
		name     string
		mockFunc func(ctx context.Context, m mocks) context.Context
		wantErr  string
	}{
		{
			name: "success notifies after store update",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.store.EXPECT().LockNamespace(gomock.Any(), name).Return(nil)
				m.notifier.EXPECT().NotifyNamespaceLocked(gomock.Any(), name)

				return ctx
			},
		},
		{
			name: "store error skips notify",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.store.EXPECT().LockNamespace(gomock.Any(), name).Return(errors.New("db error"))

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

			err := svc.Lock(ctx, name)

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

	const name = "prod"

	tests := []struct {
		name     string
		mockFunc func(ctx context.Context, m mocks) context.Context
		wantErr  string
	}{
		{
			name: "success notifies after store update",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.store.EXPECT().UnlockNamespace(gomock.Any(), name).Return(nil)
				m.notifier.EXPECT().NotifyNamespaceUnlocked(gomock.Any(), name)

				return ctx
			},
		},
		{
			name: "store error skips notify",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.store.EXPECT().UnlockNamespace(gomock.Any(), name).Return(errors.New("db error"))

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

			err := svc.Unlock(ctx, name)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}
