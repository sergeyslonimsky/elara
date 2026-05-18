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
		name        string
		description string
		mockFunc    func(ctx context.Context, m mocks) context.Context
		wantErr     string
		want        *domain.Namespace
	}{
		{
			name:        "success",
			description: "Production",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.store.EXPECT().Update(ctx, gomock.Any()).Return(nil)
				m.store.EXPECT().
					Get(ctx, name).
					Return(&domain.Namespace{Name: name, Description: "Production"}, nil)
				m.store.EXPECT().CountConfigs(ctx, name).Return(10, nil)

				return ctx
			},
			want: &domain.Namespace{Name: name, Description: "Production", ConfigCount: 10},
		},
		{
			name: "update error",
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.store.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db error"))

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

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
