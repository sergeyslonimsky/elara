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

func TestUpdateUseCase_Execute(t *testing.T) {
	t.Parallel()

	normalizedJSON := "{\n  \"key\": \"value\"\n}"

	tests := []struct {
		name     string
		input    *domain.Config
		mockFunc func(context.Context, *gomock.Controller) (*config.UpdateUseCase, context.Context)
		errIs    error
		wantErr  string
		want     *domain.Config
	}{
		{
			name: "success with format",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{"key": "value"}`,
				Format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.UpdateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockupdateEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				schemaValidator := mock_config.NewMockupdateSchemaValidator(ctrl)
				schemaValidator.EXPECT().
					Execute(ctx, "prod", "/app/config.json", normalizedJSON, domain.FormatJSON).
					Return(nil)

				configs := mock_config.NewMockconfigUpdater(ctrl)
				configs.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				namespaces := mock_config.NewMockupdateNSTimestampUpdater(ctrl)
				namespaces.EXPECT().UpdateTimestamp(ctx, "prod").Return(nil)

				watch := mock_config.NewMockupdateWatchNotifier(ctrl)
				watch.EXPECT().NotifyUpdated(ctx, gomock.Any())

				return config.NewUpdateUseCase(enforcer, configs, nil, watch, namespaces, schemaValidator), ctx
			},
			want: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   normalizedJSON,
				Format:    domain.FormatJSON,
			},
		},
		{
			name: "success detect format from existing",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{"key": "value"}`,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.UpdateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockupdateEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				getter := mock_config.NewMockupdateConfigGetter(ctrl)
				getter.EXPECT().
					Get(ctx, "/app/config.json", "prod").
					Return(&domain.Config{Format: domain.FormatJSON}, nil)

				schemaValidator := mock_config.NewMockupdateSchemaValidator(ctrl)
				schemaValidator.EXPECT().
					Execute(ctx, "prod", "/app/config.json", normalizedJSON, domain.FormatJSON).
					Return(nil)

				configs := mock_config.NewMockconfigUpdater(ctrl)
				configs.EXPECT().Update(ctx, gomock.Any()).Return(nil)

				namespaces := mock_config.NewMockupdateNSTimestampUpdater(ctrl)
				namespaces.EXPECT().UpdateTimestamp(ctx, "prod").Return(nil)

				watch := mock_config.NewMockupdateWatchNotifier(ctrl)
				watch.EXPECT().NotifyUpdated(ctx, gomock.Any())

				return config.NewUpdateUseCase(enforcer, configs, getter, watch, namespaces, schemaValidator), ctx
			},
			want: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   normalizedJSON,
				Format:    domain.FormatJSON,
			},
		},
		{
			name:  "unauthorized",
			input: &domain.Config{Namespace: "prod"},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.UpdateUseCase, context.Context) {
				return config.NewUpdateUseCase(nil, nil, nil, nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "get existing error",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{"key": "value"}`,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.UpdateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockupdateEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				getter := mock_config.NewMockupdateConfigGetter(ctrl)
				getter.EXPECT().Get(ctx, "/app/config.json", "prod").Return(nil, errors.New("not found"))

				return config.NewUpdateUseCase(enforcer, nil, getter, nil, nil, nil), ctx
			},
			wantErr: "get existing config: not found",
		},
		{
			name: "update error",
			input: &domain.Config{
				Path:      "/app/config.json",
				Namespace: "prod",
				Content:   `{"key": "value"}`,
				Format:    domain.FormatJSON,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.UpdateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockupdateEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce("user@example.com", "prod", auth.ObjectConfig, auth.ActionWrite).
					Return(true, nil)

				schemaValidator := mock_config.NewMockupdateSchemaValidator(ctrl)
				schemaValidator.EXPECT().
					Execute(ctx, "prod", "/app/config.json", normalizedJSON, domain.FormatJSON).
					Return(nil)

				configs := mock_config.NewMockconfigUpdater(ctrl)
				configs.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db error"))

				return config.NewUpdateUseCase(enforcer, configs, nil, nil, nil, schemaValidator), ctx
			},
			wantErr: "update config: db error",
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
		})
	}
}
