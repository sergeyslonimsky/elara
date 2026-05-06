package schema_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	schemamock "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestGetUseCase_Execute(t *testing.T) {
	t.Parallel()

	const (
		testEmail       = "user@example.com"
		testNamespace   = "production"
		testPathPattern = "/app/**"
	)

	now := time.Now()

	tests := []struct {
		name        string
		mockFunc    func(context.Context, *gomock.Controller) (*schema.GetUseCase, context.Context)
		namespace   string
		pathPattern string
		expected    *domain.SchemaAttachment
		errContains string
		errIs       error
	}{
		{
			name:        "success",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})
				enforcer := schemamock.NewMockschemaGetEnforcer(ctrl)
				store := schemamock.NewMockschemaGetter(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "read").Return(true, nil)
				store.EXPECT().Get(ctx, testNamespace, testPathPattern).Return(&domain.SchemaAttachment{
					ID:          "schema-1",
					Namespace:   testNamespace,
					PathPattern: testPathPattern,
					JSONSchema:  `{"type": "object"}`,
					CreatedAt:   now,
				}, nil)

				return schema.NewGetUseCase(enforcer, store), ctx
			},
			expected: &domain.SchemaAttachment{
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
			errIs:       domain.ErrUnauthorized,
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*schema.GetUseCase, context.Context) {
				return schema.NewGetUseCase(nil, nil), ctx
			},
		},
		{
			name:        "enforcer returns error wraps it",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			errContains: "enforce:",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})
				enforcer := schemamock.NewMockschemaGetEnforcer(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "read").
					Return(false, errors.New("casbin error"))

				return schema.NewGetUseCase(enforcer, nil), ctx
			},
		},
		{
			name:        "not allowed returns forbidden",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			errIs:       domain.ErrForbidden,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})
				enforcer := schemamock.NewMockschemaGetEnforcer(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "read").Return(false, nil)

				return schema.NewGetUseCase(enforcer, nil), ctx
			},
		},
		{
			name:        "store returns error wraps it",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			errContains: "get schema:",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})
				enforcer := schemamock.NewMockschemaGetEnforcer(ctrl)
				store := schemamock.NewMockschemaGetter(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "read").Return(true, nil)
				store.EXPECT().Get(ctx, testNamespace, testPathPattern).Return(nil, errors.New("bbolt error"))

				return schema.NewGetUseCase(enforcer, store), ctx
			},
		},
		{
			name:        "store returns not found propagated",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			errContains: "get schema:",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.GetUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})
				enforcer := schemamock.NewMockschemaGetEnforcer(ctrl)
				store := schemamock.NewMockschemaGetter(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "read").Return(true, nil)
				store.EXPECT().Get(ctx, testNamespace, testPathPattern).Return(nil, domain.ErrNotFound)

				return schema.NewGetUseCase(enforcer, store), ctx
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc, ctx := tt.mockFunc(t.Context(), ctrl)

			result, err := uc.Execute(ctx, tt.namespace, tt.pathPattern)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
