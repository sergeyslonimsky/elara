package schema_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	mock_schema "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

func TestGetEffectiveUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		mockFunc func(context.Context, *gomock.Controller) (*schema.GetEffectiveUseCase, context.Context)
		errIs    error
		want     *domain.SchemaAttachment
	}{
		{
			name: "success best match",
			path: "/app/config.json",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.GetEffectiveUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_schema.NewMockgetEffectiveEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "schema", "read").Return(true, nil)

				repo := mock_schema.NewMockschemaContentLister(ctrl)
				now := time.Now()
				schemas := []*domain.SchemaAttachment{
					{ID: "s1", PathPattern: "/*.json", CreatedAt: now},
					{ID: "s2", PathPattern: "/app/*.json", CreatedAt: now},
				}
				repo.EXPECT().List(ctx, "prod").Return(schemas, nil)

				return schema.NewGetEffectiveUseCase(enforcer, repo), ctx
			},
			want: &domain.SchemaAttachment{ID: "s2", PathPattern: "/app/*.json"},
		},
		{
			name: "no match",
			path: "/other.txt",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*schema.GetEffectiveUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_schema.NewMockgetEffectiveEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "schema", "read").Return(true, nil)
				repo := mock_schema.NewMockschemaContentLister(ctrl)
				repo.EXPECT().List(ctx, "prod").Return([]*domain.SchemaAttachment{}, nil)

				return schema.NewGetEffectiveUseCase(enforcer, repo), ctx
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, "prod", tt.path)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

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
