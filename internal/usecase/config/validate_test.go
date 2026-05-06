package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	mock_config "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

func TestValidateUseCase_Execute(t *testing.T) {
	t.Parallel()

	normalizedJSON := "{\n  \"key\": \"value\"\n}"

	tests := []struct {
		name      string
		content   string
		format    domain.Format
		namespace string
		path      string
		mockFunc  func(context.Context, *gomock.Controller) (*config.ValidateUseCase, context.Context)
		errIs     error
		wantErr   string
		want      *domain.ValidationResult
	}{
		{
			name:      "success with schema",
			content:   `{"key": "value"}`,
			format:    domain.FormatJSON,
			namespace: "prod",
			path:      "/a.json",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.ValidateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockvalidateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)

				schema := mock_config.NewMockvalidateSchemaChecker(ctrl)
				schema.EXPECT().Execute(ctx, "prod", "/a.json", normalizedJSON, domain.FormatJSON).Return(nil)

				return config.NewValidateUseCase(enforcer, schema), ctx
			},
			want: &domain.ValidationResult{
				Valid:             true,
				DetectedFormat:    domain.FormatJSON,
				NormalizedContent: normalizedJSON,
			},
		},
		{
			name:    "success without schema (no namespace)",
			content: `{"key": "value"}`,
			format:  domain.FormatJSON,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.ValidateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				return config.NewValidateUseCase(nil, nil), ctx
			},
			want: &domain.ValidationResult{
				Valid:             true,
				DetectedFormat:    domain.FormatJSON,
				NormalizedContent: normalizedJSON,
			},
		},
		{
			name:    "unauthorized",
			content: `{}`,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.ValidateUseCase, context.Context) {
				return config.NewValidateUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:    "invalid json",
			content: `{invalid}`,
			format:  domain.FormatJSON,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.ValidateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				return config.NewValidateUseCase(nil, nil), ctx
			},
			want: &domain.ValidationResult{
				Valid:          false,
				DetectedFormat: domain.FormatJSON,
				Errors:         []string{"unmarshal JSON: invalid character 'i' looking for beginning of object key string"},
			},
		},
		{
			name:      "schema violation",
			content:   `{"key": "value"}`,
			format:    domain.FormatJSON,
			namespace: "prod",
			path:      "/a.json",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*config.ValidateUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})

				enforcer := mock_config.NewMockvalidateEnforcer(ctrl)
				enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)

				schema := mock_config.NewMockvalidateSchemaChecker(ctrl)
				err := &domain.SchemaValidationError{
					Violations: []domain.SchemaViolation{{Message: "too long"}},
				}
				schema.EXPECT().Execute(ctx, "prod", "/a.json", normalizedJSON, domain.FormatJSON).Return(err)

				return config.NewValidateUseCase(enforcer, schema), ctx
			},
			want: &domain.ValidationResult{
				Valid:             false,
				DetectedFormat:    domain.FormatJSON,
				NormalizedContent: normalizedJSON,
				SchemaViolations:  []domain.SchemaViolation{{Message: "too long"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.content, tt.format, tt.namespace, tt.path)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.Valid, got.Valid)
			assert.Equal(t, tt.want.DetectedFormat, got.DetectedFormat)
			assert.Equal(t, tt.want.NormalizedContent, got.NormalizedContent)
			if len(tt.want.Errors) > 0 {
				require.NotEmpty(t, got.Errors)
				assert.Contains(t, got.Errors[0], tt.want.Errors[0])
			}
			assert.Equal(t, tt.want.SchemaViolations, got.SchemaViolations)
		})
	}
}
