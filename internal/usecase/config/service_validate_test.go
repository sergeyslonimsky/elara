package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

func TestService_Validate(t *testing.T) {
	t.Parallel()

	normalizedJSON := "{\n  \"key\": \"value\"\n}"

	tests := []struct {
		name     string
		input    config.ValidateInput
		mockFunc func(ctx context.Context, m mocks) context.Context
		errIs    error
		wantErr  string
		want     *domain.ValidationResult
	}{
		{
			name: "success with schema",
			input: config.ValidateInput{
				Content:   `{"key": "value"}`,
				Format:    domain.FormatJSON,
				Namespace: "prod",
				Path:      "/a.json",
			},
			mockFunc: func(ctx context.Context, m mocks) context.Context {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				m.enforcer.EXPECT().Enforce("user@example.com", "prod", "config", "read").Return(true, nil)
				m.schemaValidator.EXPECT().
					Validate(ctx, "prod", "/a.json", normalizedJSON, domain.FormatJSON).
					Return(nil)

				return ctx
			},
			want: &domain.ValidationResult{Valid: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			ctx := tt.mockFunc(t.Context(), m)

			got, err := svc.Validate(ctx, tt.input)

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
		})
	}
}
