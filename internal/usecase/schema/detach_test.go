package schema_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	mockschema "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestDetachUseCase_Execute(t *testing.T) {
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
		mockFunc    func(context.Context, *gomock.Controller) (*schema.DetachUseCase, context.Context)
		wantErr     string
	}{
		{
			name:        "success",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.DetachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockdetachEnforcer(ctrl)
				store := mockschema.NewMockschemaDetacher(ctrl)
				namespaces := mockschema.NewMockdetachNSChecker(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)
				store.EXPECT().Detach(ctx, testNamespace, testPathPattern).Return(nil)

				return schema.NewDetachUseCase(enforcer, store, namespaces), ctx
			},
		},
		{
			name:        "unauthorized when no claims in context",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.DetachUseCase, context.Context) {
				return schema.NewDetachUseCase(nil, nil, nil), ctx
			},
			wantErr: domain.ErrUnauthorized.Error(),
		},
		{
			name:        "enforce error wraps error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.DetachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockdetachEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce(testEmail, testNamespace, "schema", "write").
					Return(false, errors.New("casbin fail"))

				return schema.NewDetachUseCase(enforcer, nil, nil), ctx
			},
			wantErr: "enforce: casbin fail",
		},
		{
			name:        "forbidden when enforcer returns false",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.DetachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockdetachEnforcer(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(false, nil)

				return schema.NewDetachUseCase(enforcer, nil, nil), ctx
			},
			wantErr: domain.ErrForbidden.Error(),
		},
		{
			name:        "get namespace error wraps error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.DetachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockdetachEnforcer(ctrl)
				namespaces := mockschema.NewMockdetachNSChecker(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().Get(ctx, testNamespace).Return(nil, errors.New("db error"))

				return schema.NewDetachUseCase(enforcer, nil, namespaces), ctx
			},
			wantErr: "get namespace: db error",
		},
		{
			name:        "namespace locked returns error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.DetachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockdetachEnforcer(ctrl)
				namespaces := mockschema.NewMockdetachNSChecker(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: true}, nil)

				return schema.NewDetachUseCase(enforcer, nil, namespaces), ctx
			},
			wantErr: domain.ErrNamespaceLocked.Error(),
		},
		{
			name:        "detach error wraps error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.DetachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockdetachEnforcer(ctrl)
				store := mockschema.NewMockschemaDetacher(ctrl)
				namespaces := mockschema.NewMockdetachNSChecker(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)
				store.EXPECT().Detach(ctx, testNamespace, testPathPattern).Return(errors.New("store fail"))

				return schema.NewDetachUseCase(enforcer, store, namespaces), ctx
			},
			wantErr: "detach schema: store fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Execute(ctx, tt.namespace, tt.pathPattern)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}
