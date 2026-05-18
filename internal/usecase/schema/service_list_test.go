package schema_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	schemamock "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestService_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace string
		mockFunc  func(context.Context, *gomock.Controller) (*schema.Service, context.Context)
		errIs     error
		wantErr   string
		want      []*domain.SchemaAttachment
	}{
		{
			name:      "success",
			namespace: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := schemamock.NewMockenforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "schema", "read").Return(true, nil)

				store := schemamock.NewMockstore(ctrl)
				list := []*domain.SchemaAttachment{{ID: "s1"}}
				store.EXPECT().List(ctx, "prod").Return(list, nil)

				return schema.New(enforcer, store, nil), ctx
			},
			want: []*domain.SchemaAttachment{{ID: "s1"}},
		},
		{
			name:      "forbidden returns nil",
			namespace: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := schemamock.NewMockenforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "schema", "read").Return(false, nil)

				return schema.New(enforcer, nil, nil), ctx
			},
			want: nil,
		},
		{
			name:      "unauthorized",
			namespace: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				return schema.New(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:      "list error",
			namespace: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := schemamock.NewMockenforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "schema", "read").Return(true, nil)
				store := schemamock.NewMockstore(ctrl)
				store.EXPECT().List(ctx, "prod").Return(nil, errors.New("db error"))

				return schema.New(enforcer, store, nil), ctx
			},
			wantErr: "list schemas: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.List(ctx, tt.namespace)

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
