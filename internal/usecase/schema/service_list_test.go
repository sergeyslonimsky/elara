package schema_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	schemamock "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

const testUserID = "11111111-2222-3333-4444-555555555555"

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
				ctx = auth2.WithSession(
					ctx,
					&domain.Session{},
					&domain.User{ID: uuid.MustParse(testUserID), Email: "user@example.com"},
				)

				pdp := schemamock.NewMockpdp(ctrl)
				pdp.EXPECT().HasNamespace(testUserID, "prod", domain.ActionRead).Return(true)

				store := schemamock.NewMockstore(ctrl)
				list := []*domain.SchemaAttachment{{ID: "s1"}}
				store.EXPECT().List(ctx, "prod").Return(list, nil)

				return schema.New(pdp, store, nil), ctx
			},
			want: []*domain.SchemaAttachment{{ID: "s1"}},
		},
		{
			name:      "forbidden returns nil",
			namespace: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth2.WithSession(
					ctx,
					&domain.Session{},
					&domain.User{ID: uuid.MustParse(testUserID), Email: "user@example.com"},
				)

				pdp := schemamock.NewMockpdp(ctrl)
				pdp.EXPECT().HasNamespace(testUserID, "prod", domain.ActionRead).Return(false)

				return schema.New(pdp, nil, nil), ctx
			},
			want: nil,
		},
		{
			name:      "unauthorized",
			namespace: "prod",
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*schema.Service, context.Context) {
				return schema.New(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:      "list error",
			namespace: "prod",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth2.WithSession(
					ctx,
					&domain.Session{},
					&domain.User{ID: uuid.MustParse(testUserID), Email: "user@example.com"},
				)

				pdp := schemamock.NewMockpdp(ctrl)
				pdp.EXPECT().HasNamespace(testUserID, "prod", domain.ActionRead).Return(true)

				store := schemamock.NewMockstore(ctrl)
				store.EXPECT().List(ctx, "prod").Return(nil, errors.New("db error"))

				return schema.New(pdp, store, nil), ctx
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
