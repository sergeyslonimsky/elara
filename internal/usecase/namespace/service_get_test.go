package namespace_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Authorization is enforced at the handler boundary; these tests cover only
// the business behaviour of Get.

func TestService_Get(t *testing.T) {
	t.Parallel()

	const name = "prod"

	tests := []struct {
		name     string
		mockFunc func(ctx context.Context, m mocks) context.Context
		wantErr  string
		want     *domain.Namespace
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.store.EXPECT().Get(ctx, name).Return(&domain.Namespace{Name: name}, nil)
				m.store.EXPECT().CountConfigs(ctx, name).Return(5, nil)

				return ctx
			},
			want: &domain.Namespace{Name: name, ConfigCount: 5},
		},
		{
			name: "namespace not found",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.store.EXPECT().Get(ctx, name).Return(nil, domain.ErrNotFound)

				return ctx
			},
			wantErr: "get namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)
			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.Get(ctx, name)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
