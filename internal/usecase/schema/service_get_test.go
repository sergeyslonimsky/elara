package schema_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	schemamock "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestService_Get(t *testing.T) {
	t.Parallel()

	const (
		testEmail       = "user@example.com"
		testNamespace   = "production"
		testPathPattern = "/app/**"
	)

	now := time.Now()

	tests := []struct {
		name        string
		namespace   string
		pathPattern string
		mockFunc    func(context.Context, *gomock.Controller) (*schema.Service, context.Context)
		errIs       error
		wantErr     string
		want        *domain.SchemaAttachment
	}{
		{
			name:        "success",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})
				enforcer := schemamock.NewMockenforcer(ctrl)
				store := schemamock.NewMockstore(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "read").Return(true, nil)
				store.EXPECT().Get(ctx, testNamespace, testPathPattern).Return(&domain.SchemaAttachment{
					ID:          "schema-1",
					Namespace:   testNamespace,
					PathPattern: testPathPattern,
					JSONSchema:  `{"type": "object"}`,
					CreatedAt:   now,
				}, nil)

				return schema.New(enforcer, store, nil), ctx
			},
			want: &domain.SchemaAttachment{
				ID:          "schema-1",
				Namespace:   testNamespace,
				PathPattern: testPathPattern,
				JSONSchema:  `{"type": "object"}`,
				CreatedAt:   now,
			},
		},
		{
			name:        "no claims in context returns unauthorized",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*schema.Service, context.Context) {
				return schema.New(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:        "enforcer returns error wraps it",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})
				enforcer := schemamock.NewMockenforcer(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "read").
					Return(false, errors.New("casbin error"))

				return schema.New(enforcer, nil, nil), ctx
			},
			wantErr: "enforce:",
		},
		{
			name:        "not allowed returns forbidden",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})
				enforcer := schemamock.NewMockenforcer(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "read").Return(false, nil)

				return schema.New(enforcer, nil, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:        "store returns error wraps it",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})
				enforcer := schemamock.NewMockenforcer(ctrl)
				store := schemamock.NewMockstore(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "read").Return(true, nil)
				store.EXPECT().Get(ctx, testNamespace, testPathPattern).Return(nil, errors.New("bbolt error"))

				return schema.New(enforcer, store, nil), ctx
			},
			wantErr: "get schema:",
		},
		{
			name:        "store returns not found propagated",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})
				enforcer := schemamock.NewMockenforcer(ctrl)
				store := schemamock.NewMockstore(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "read").Return(true, nil)
				store.EXPECT().Get(ctx, testNamespace, testPathPattern).Return(nil, domain.ErrNotFound)

				return schema.New(enforcer, store, nil), ctx
			},
			wantErr: "get schema:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Get(ctx, tt.namespace, tt.pathPattern)

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
