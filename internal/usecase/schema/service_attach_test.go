package schema_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	schemamock "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestService_Attach(t *testing.T) {
	t.Parallel()

	const (
		testNamespace   = "production"
		testPathPattern = "/app/**"
		testJSONSchema  = `{"type": "object"}`
	)

	tests := []struct {
		name     string
		input    schema.AttachInput
		mockFunc func(context.Context, *gomock.Controller) (*schema.Service, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			input: schema.AttachInput{
				Namespace:   testNamespace,
				PathPattern: testPathPattern,
				JSONSchema:  testJSONSchema,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				store := schemamock.NewMockstore(ctrl)
				namespaces := schemamock.NewMocknsProvider(ctrl)

				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)
				store.EXPECT().Attach(ctx, gomock.Any()).Return(nil)

				return schema.New(nil, store, namespaces), ctx
			},
		},
		{
			name: "get namespace error wraps error",
			input: schema.AttachInput{
				Namespace:   testNamespace,
				PathPattern: testPathPattern,
				JSONSchema:  testJSONSchema,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				namespaces := schemamock.NewMocknsProvider(ctrl)
				namespaces.EXPECT().Get(ctx, testNamespace).Return(nil, errors.New("db error"))

				return schema.New(nil, nil, namespaces), ctx
			},
			wantErr: "get namespace: db error",
		},
		{
			name: "namespace locked error",
			input: schema.AttachInput{
				Namespace:   testNamespace,
				PathPattern: testPathPattern,
				JSONSchema:  testJSONSchema,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				namespaces := schemamock.NewMocknsProvider(ctrl)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: true}, nil)

				return schema.New(nil, nil, namespaces), ctx
			},
			errIs: domain.ErrNamespaceLocked,
		},
		{
			name: "invalid json schema error",
			input: schema.AttachInput{
				Namespace:   testNamespace,
				PathPattern: testPathPattern,
				JSONSchema:  `invalid`,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				namespaces := schemamock.NewMocknsProvider(ctrl)
				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)

				return schema.New(nil, nil, namespaces), ctx
			},
			wantErr: "validate json schema",
		},
		{
			name: "attach error wraps error",
			input: schema.AttachInput{
				Namespace:   testNamespace,
				PathPattern: testPathPattern,
				JSONSchema:  testJSONSchema,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				store := schemamock.NewMockstore(ctrl)
				namespaces := schemamock.NewMocknsProvider(ctrl)

				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)
				store.EXPECT().Attach(ctx, gomock.Any()).Return(errors.New("store failure"))

				return schema.New(nil, store, namespaces), ctx
			},
			wantErr: "attach schema: store failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Attach(ctx, tt.input)

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
			assert.Equal(t, tt.input.Namespace, got.Namespace)
			assert.Equal(t, tt.input.PathPattern, got.PathPattern)
			assert.Equal(t, tt.input.JSONSchema, got.JSONSchema)
		})
	}
}
