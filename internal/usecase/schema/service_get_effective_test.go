package schema_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	schemamock "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestService_GetEffective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace string
		path      string
		mockFunc  func(context.Context, *gomock.Controller) (*schema.Service, context.Context)
		errIs     error
		wantErr   string
		want      *domain.SchemaAttachment
	}{
		{
			name:      "success best match",
			namespace: "prod",
			path:      "/app/config.json",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				store := schemamock.NewMockstore(ctrl)
				now := time.Now()
				schemas := []*domain.SchemaAttachment{
					{ID: "s1", PathPattern: "/*.json", CreatedAt: now},
					{ID: "s2", PathPattern: "/app/*.json", CreatedAt: now},
				}
				store.EXPECT().List(ctx, "prod").Return(schemas, nil)

				return schema.New(nil, store, nil), ctx
			},
			want: &domain.SchemaAttachment{ID: "s2", PathPattern: "/app/*.json"},
		},
		{
			name:      "no match",
			namespace: "prod",
			path:      "/other.txt",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.Service, context.Context) {
				store := schemamock.NewMockstore(ctrl)
				store.EXPECT().List(ctx, "prod").Return([]*domain.SchemaAttachment{}, nil)

				return schema.New(nil, store, nil), ctx
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.GetEffective(ctx, tt.namespace, tt.path)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.want.ID, got.ID)
			}
		})
	}
}
