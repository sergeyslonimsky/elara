package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Authorization is enforced at the handler boundary; these tests cover only
// the business behaviour of Delete (non-empty guard + store error).

func TestService_Delete(t *testing.T) {
	t.Parallel()

	const name = "prod"

	tests := []struct {
		name     string
		mockFunc func(ctx context.Context, m mocks) context.Context
		wantErr  string
	}{
		{
			name: "success when namespace is empty",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.store.EXPECT().
					Get(gomock.Any(), name).
					Return(&domain.Namespace{Name: name}, nil)
				m.store.EXPECT().CountConfigs(gomock.Any(), name).Return(0, nil)
				m.store.EXPECT().Delete(gomock.Any(), name).Return(nil)

				return ctx
			},
		},
		{
			name: "rejects deletion when namespace still has configs",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.store.EXPECT().
					Get(gomock.Any(), name).
					Return(&domain.Namespace{Name: name}, nil)
				m.store.EXPECT().CountConfigs(gomock.Any(), name).Return(5, nil)

				return ctx
			},
			wantErr: `namespace "prod" contains 5 config(s)`,
		},
		{
			name: "rejects deletion when namespace is locked",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.store.EXPECT().
					Get(gomock.Any(), name).
					Return(&domain.Namespace{Name: name, Locked: true}, nil)

				return ctx
			},
			wantErr: domain.ErrNamespaceLocked.Error(),
		},
		{
			name: "delete store error",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.store.EXPECT().
					Get(gomock.Any(), name).
					Return(&domain.Namespace{Name: name}, nil)
				m.store.EXPECT().CountConfigs(gomock.Any(), name).Return(0, nil)
				m.store.EXPECT().Delete(gomock.Any(), name).Return(errors.New("db error"))

				return ctx
			},
			wantErr: "delete namespace: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			err := svc.Delete(ctx, name)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}
