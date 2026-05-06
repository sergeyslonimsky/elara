package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	mock_config "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

func TestCreateUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *domain.Config
		mockFunc func(context.Context, *gomock.Controller) (*config.CreateUseCase, context.Context)
		errIs    error
		wantErr  string
		want     *domain.Config
	}{
		{
			name: "success",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{"key": "value"}`,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.CreateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockcreateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).Return(true, nil)

				nsChecker := mock_config.NewMockcreateNSChecker(ctrl)
				nsChecker.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				schemaValidator := mock_config.NewMockcreateSchemaValidator(ctrl)
				normalized := "{\n  \"key\": \"value\"\n}"
				schemaValidator.EXPECT().Execute(ctx, "prod", "/app/config.json", normalized, domain.FormatJSON).Return(nil)

				configs := mock_config.NewMockconfigCreator(ctrl)
				configs.EXPECT().Create(ctx, gomock.Any()).Return(nil)

				namespaces := mock_config.NewMockcreateNSTimestampUpdater(ctrl)
				namespaces.EXPECT().UpdateTimestamp(ctx, "prod").Return(nil)

				watch := mock_config.NewMockcreateWatchNotifier(ctrl)
				watch.EXPECT().NotifyCreated(ctx, gomock.Any())

				return config.NewCreateUseCase(enforcer, configs, watch, namespaces, nsChecker, schemaValidator), ctx
			},
			want: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   "{\n  \"key\": \"value\"\n}",
				Format:    domain.FormatJSON,
				Version:   1,
			},
		},
		{
			name:  "unauthorized",
			input: &domain.Config{Namespace: "prod"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.CreateUseCase, context.Context) {
				return config.NewCreateUseCase(nil, nil, nil, nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:  "invalid path",
			input: &domain.Config{Path: "invalid", Namespace: "prod"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.CreateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_config.NewMockcreateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).Return(true, nil)
				return config.NewCreateUseCase(enforcer, nil, nil, nil, nil, nil), ctx
			},
			wantErr: "validate path",
		},
		{
			name:  "namespace does not exist",
			input: &domain.Config{Path: "/app/config.json", Namespace: "prod"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.CreateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_config.NewMockcreateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).Return(true, nil)

				nsChecker := mock_config.NewMockcreateNSChecker(ctrl)
				nsChecker.EXPECT().Get(ctx, "prod").Return(nil, domain.ErrNotFound)

				return config.NewCreateUseCase(enforcer, nil, nil, nil, nsChecker, nil), ctx
			},
			wantErr: `namespace "prod" does not exist`,
		},
		{
			name: "invalid content",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{invalid json}`,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.CreateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_config.NewMockcreateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).Return(true, nil)

				nsChecker := mock_config.NewMockcreateNSChecker(ctrl)
				nsChecker.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				return config.NewCreateUseCase(enforcer, nil, nil, nil, nsChecker, nil), ctx
			},
			wantErr: "validate content",
		},
		{
			name: "schema validation error",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{"key": "value"}`,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.CreateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_config.NewMockcreateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).Return(true, nil)

				nsChecker := mock_config.NewMockcreateNSChecker(ctrl)
				nsChecker.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				schemaValidator := mock_config.NewMockcreateSchemaValidator(ctrl)
				normalized := "{\n  \"key\": \"value\"\n}"
				schemaValidator.EXPECT().Execute(ctx, "prod", "/app/config.json", normalized, domain.FormatJSON).Return(errors.New("schema error"))

				return config.NewCreateUseCase(enforcer, nil, nil, nil, nsChecker, schemaValidator), ctx
			},
			wantErr: "schema validation: schema error",
		},
		{
			name: "create config error",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{"key": "value"}`,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.CreateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := mock_config.NewMockcreateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).Return(true, nil)

				nsChecker := mock_config.NewMockcreateNSChecker(ctrl)
				nsChecker.EXPECT().Get(ctx, "prod").Return(&domain.Namespace{Name: "prod"}, nil)

				schemaValidator := mock_config.NewMockcreateSchemaValidator(ctrl)
				normalized := "{\n  \"key\": \"value\"\n}"
				schemaValidator.EXPECT().Execute(ctx, "prod", "/app/config.json", normalized, domain.FormatJSON).Return(nil)

				configs := mock_config.NewMockconfigCreator(ctrl)
				configs.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))

				return config.NewCreateUseCase(enforcer, configs, nil, nil, nsChecker, schemaValidator), ctx
			},
			wantErr: "create config: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.input)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.Path, got.Path)
			assert.Equal(t, tt.want.Namespace, got.Namespace)
			assert.Equal(t, tt.want.Content, got.Content)
			assert.Equal(t, tt.want.Format, got.Format)
			assert.Equal(t, tt.want.Version, got.Version)
		})
	}
}
