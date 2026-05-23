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

// Authorization (Namespace, Create, *) is enforced in the handler (EL-4 M9);
// this test covers the remaining usecase logic only.

func TestService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *domain.Namespace
		mockFunc func(ctx context.Context, m mocks) context.Context
		wantErr  string
		want     *domain.Namespace
	}{
		{
			name:  "success",
			input: &domain.Namespace{Name: "prod"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				m.store.EXPECT().Create(ctx, gomock.Any()).Return(nil)
				m.store.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				return ctx
			},
			want: &domain.Namespace{Name: "prod"},
		},
		{
			name:  "validation error",
			input: &domain.Namespace{Name: ""},
			mockFunc: func(ctx context.Context, _ mocks) context.Context {
				return ctx
			},
			wantErr: "validate namespace",
		},
		{
			name:  "create error",
			input: &domain.Namespace{Name: "prod"},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
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

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
