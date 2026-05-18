package namespace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

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
				m.authz.EXPECT().
					Require(ctx, domain.ObjectNamespace, domain.ActionWrite, name).
					Return(nil)
				m.store.EXPECT().CountConfigs(ctx, name).Return(0, nil)
				m.store.EXPECT().Delete(ctx, name).Return(nil)

				return ctx
			},
		},
		{
			name: "rejects deletion when namespace still has configs",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.authz.EXPECT().
					Require(ctx, domain.ObjectNamespace, domain.ActionWrite, name).
					Return(nil)
				m.store.EXPECT().CountConfigs(ctx, name).Return(5, nil)

				return ctx
			},
			wantErr: `namespace "prod" contains 5 config(s)`,
		},
		{
			name: "delete store error",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.authz.EXPECT().
					Require(ctx, domain.ObjectNamespace, domain.ActionWrite, name).
					Return(nil)
				m.store.EXPECT().CountConfigs(ctx, name).Return(0, nil)
				m.store.EXPECT().Delete(ctx, name).Return(errors.New("db error"))

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
