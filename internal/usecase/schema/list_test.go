package schema_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	mock_schema "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestListUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ns       string
		mockFunc func(context.Context, *gomock.Controller) (*schema.ListUseCase, context.Context)
		errIs    error
		wantErr  string
		want     []*domain.SchemaAttachment
	}{
		{
			name: "success",
			ns:   "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.ListUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_schema.NewMockschemaListEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "schema", "read").Return(true, nil)

				store := mock_schema.NewMockschemaLister(ctrl)
				list := []*domain.SchemaAttachment{{ID: "s1"}}
				store.EXPECT().List(ctx, "prod").Return(list, nil)

				return schema.NewListUseCase(enforcer, store), ctx
			},
			want: []*domain.SchemaAttachment{{ID: "s1"}},
		},
		{
			name: "forbidden returns nil",
			ns:   "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.ListUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_schema.NewMockschemaListEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "schema", "read").Return(false, nil)
				return schema.NewListUseCase(enforcer, nil), ctx
			},
			want: nil,
		},
		{
			name: "unauthorized",
			ns:   "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.ListUseCase, context.Context) {
				return schema.NewListUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "list error",
			ns:   "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.ListUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_schema.NewMockschemaListEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "schema", "read").Return(true, nil)
				store := mock_schema.NewMockschemaLister(ctrl)
				store.EXPECT().List(ctx, "prod").Return(nil, errors.New("db error"))
				return schema.NewListUseCase(enforcer, store), ctx
			},
			wantErr: "list schemas: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.ns)

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
