package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Authorization is enforced at the handler boundary; these tests cover only
// the business behaviour of Update.

func TestService_Update(t *testing.T) {
	t.Parallel()

	const name = "prod"

	tests := []struct {
		name            string
		description     string
		mockFunc        func(ctx context.Context, m mocks) context.Context
		errIs           error
		wantErr         string
		wantDescription string
		wantCount       int
	}{
		{
			name:        "success",
			description: "Production",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.store.EXPECT().
					Get(gomock.Any(), name).
					Return(&domain.Namespace{Name: name, Description: "old"}, nil)
				m.store.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
				m.store.EXPECT().CountConfigs(gomock.Any(), name).Return(10, nil)

				return ctx
			},
			wantDescription: "Production",
			wantCount:       10,
		},
		{
			name:        "locked returns ErrNamespaceLocked",
			description: "x",
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
			errIs: domain.ErrNamespaceLocked,
		},
		{
			name: "update error",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.store.EXPECT().
					Get(gomock.Any(), name).
					Return(&domain.Namespace{Name: name}, nil)
				m.store.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

				return ctx
			},
			wantErr: "update namespace: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.Update(ctx, name, tt.description)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantDescription, got.Description)
			assert.Equal(t, tt.wantCount, got.ConfigCount)
			assert.False(t, got.UpdatedAt.IsZero(), "service should set UpdatedAt")
		})
	}
}
