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
	mockschema "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestAttachUseCase_Execute(t *testing.T) {
	t.Parallel()

	const (
		testEmail       = "user@example.com"
		testNamespace   = "production"
		testPathPattern = "/app/**"
		testJSONSchema  = `{"type": "object"}`
	)

	tests := []struct {
		name        string
		namespace   string
		pathPattern string
		jsonSchema  string
		mockFunc    func(context.Context, *gomock.Controller) (*schema.AttachUseCase, context.Context)
		errIs       error
		wantErr     string
	}{
		{
			name:        "success",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			jsonSchema:  testJSONSchema,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.AttachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockattachEnforcer(ctrl)
				store := mockschema.NewMockschemaAttacher(ctrl)
				namespaces := mockschema.NewMockattachNSChecker(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)

				store.EXPECT().Attach(ctx, gomock.Any()).Return(nil)

				return schema.NewAttachUseCase(enforcer, store, namespaces), ctx
			},
		},
		{
			name:        "unauthorized when no claims in context",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			jsonSchema:  testJSONSchema,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.AttachUseCase, context.Context) {
				return schema.NewAttachUseCase(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:        "enforce error wraps error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			jsonSchema:  testJSONSchema,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.AttachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockattachEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce(testEmail, testNamespace, "schema", "write").
					Return(false, errors.New("casbin fail"))

				return schema.NewAttachUseCase(enforcer, nil, nil), ctx
			},
			wantErr: "enforce: casbin fail",
		},
		{
			name:        "forbidden when enforcer returns false",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			jsonSchema:  testJSONSchema,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.AttachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockattachEnforcer(ctrl)
				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(false, nil)

				return schema.NewAttachUseCase(enforcer, nil, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:        "get namespace error wraps error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			jsonSchema:  testJSONSchema,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.AttachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockattachEnforcer(ctrl)
				namespaces := mockschema.NewMockattachNSChecker(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(nil, errors.New("db error"))

				return schema.NewAttachUseCase(enforcer, nil, namespaces), ctx
			},
			wantErr: "get namespace: db error",
		},
		{
			name:        "namespace locked error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			jsonSchema:  testJSONSchema,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.AttachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockattachEnforcer(ctrl)
				namespaces := mockschema.NewMockattachNSChecker(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: true}, nil)

				return schema.NewAttachUseCase(enforcer, nil, namespaces), ctx
			},
			wantErr: "namespace \"production\": namespace is locked: config is locked",
		},
		{
			name:        "invalid json schema error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			jsonSchema:  `invalid`,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.AttachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockattachEnforcer(ctrl)
				namespaces := mockschema.NewMockattachNSChecker(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)

				return schema.NewAttachUseCase(enforcer, nil, namespaces), ctx
			},
			wantErr: "validate json schema: validation: json_schema: invalid JSON",
		},
		{
			name:        "attach error wraps error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			jsonSchema:  testJSONSchema,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.AttachUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: testEmail})

				enforcer := mockschema.NewMockattachEnforcer(ctrl)
				store := mockschema.NewMockschemaAttacher(ctrl)
				namespaces := mockschema.NewMockattachNSChecker(ctrl)

				enforcer.EXPECT().Enforce(testEmail, testNamespace, "schema", "write").Return(true, nil)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)

				store.EXPECT().Attach(ctx, gomock.Any()).Return(errors.New("store failure"))

				return schema.NewAttachUseCase(enforcer, store, namespaces), ctx
			},
			wantErr: "attach schema: store failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.namespace, tt.pathPattern, tt.jsonSchema)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, got.ID)
			assert.Equal(t, tt.namespace, got.Namespace)
			assert.Equal(t, tt.pathPattern, got.PathPattern)
			assert.Equal(t, tt.jsonSchema, got.JSONSchema)
		})
	}
}
