package schema_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	schemamock "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestService_Detach(t *testing.T) {
	t.Parallel()

	const (
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
				store := schemamock.NewMockstore(ctrl)
				namespaces := schemamock.NewMocknsProvider(ctrl)

				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)
				store.EXPECT().Detach(ctx, testNamespace, testPathPattern).Return(nil)

				return schema.New(nil, store, namespaces), ctx
			},
		},
		{
			name:        "get namespace error wraps error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				namespaces := schemamock.NewMocknsProvider(ctrl)
				namespaces.EXPECT().Get(ctx, testNamespace).Return(nil, errors.New("db error"))

				return schema.New(nil, nil, namespaces), ctx
			},
			wantErr: "get namespace: db error",
		},
		{
			name:        "namespace locked returns error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
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
			name:        "detach error wraps error",
			namespace:   testNamespace,
			pathPattern: testPathPattern,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				store := schemamock.NewMockstore(ctrl)
				namespaces := schemamock.NewMocknsProvider(ctrl)

				namespaces.EXPECT().
					Get(ctx, testNamespace).
					Return(&domain.Namespace{Name: testNamespace, Locked: false}, nil)
				store.EXPECT().
					Detach(ctx, testNamespace, testPathPattern).
					Return(errors.New("store fail"))

				return schema.New(nil, store, namespaces), ctx
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
