package schema_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	schema_mock "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestService_Detach(t *testing.T) {
	t.Parallel()

	const (
		testEmail       = "user@example.com"
		testNamespace   = "production"
		testPathPattern = "/app/**"
	)

	tests := []struct {
		name        string
		namespace   string
		pathPattern string
		mockFunc    func(context.Context, *gomock.Controller) (*schema.Service, context.Context)
		errIs       error
		wantErr     string
	}{
		{
			name:        "success",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := schema_mock.NewMockenforcer(ctrl)
				store := schema_mock.NewMockstore(ctrl)
				namespaces := schema_mock.NewMocknsProvider(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)
				store.EXPECT().Detach(ctx, testNamespace, testPathPattern).Return(nil)

				return schema.New(enforcer, store, namespaces), ctx
			},
		},
		{
			name:        "unauthorized when no claims in context",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				return schema.New(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:        "enforce error wraps error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := schema_mock.NewMockenforcer(ctrl)
				enforcer.EXPECT().
					Enforce(testEmail, testNamespace, "schema", "write").
					Return(false, errors.New("casbin fail"))

				return schema.New(enforcer, nil, nil), ctx
			},
			wantErr: "enforce: casbin fail",
		},
		{
			name:        "forbidden when enforcer returns false",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := schema_mock.NewMockenforcer(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(false, nil)

				return schema.New(enforcer, nil, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:        "get namespace error wraps error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := schema_mock.NewMockenforcer(ctrl)
				namespaces := schema_mock.NewMocknsProvider(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().Get(ctx, testNamespace).Return(nil, errors.New("db error"))

				return schema.New(enforcer, nil, namespaces), ctx
			},
			wantErr: "get namespace: db error",
		},
		{
			name:        "namespace locked returns error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := schema_mock.NewMockenforcer(ctrl)
				namespaces := schema_mock.NewMocknsProvider(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: true}, nil)

				return schema.New(enforcer, nil, namespaces), ctx
			},
			errIs: domain.ErrNamespaceLocked,
		},
		{
			name:        "detach error wraps error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := schema_mock.NewMockenforcer(ctrl)
				store := schema_mock.NewMockstore(ctrl)
				namespaces := schema_mock.NewMocknsProvider(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)
				store.EXPECT().Detach(ctx, testNamespace, testPathPattern).Return(errors.New("store fail"))

				return schema.New(enforcer, store, namespaces), ctx
			},
			wantErr: "detach schema: store fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Detach(ctx, tt.namespace, tt.pathPattern)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}
