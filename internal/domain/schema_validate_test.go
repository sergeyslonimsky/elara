package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestValidateJSONSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		schema    string
		wantError bool
	}{
		{
			name:      "valid object schema",
			schema:    `{"type": "object", "properties": {"host": {"type": "string"}}}`,
			wantError: false,
		},
		{
			name:      "valid boolean schema",
			schema:    `true`,
			wantError: false,
		},
		{
			name:      "invalid JSON",
			schema:    `{not valid json}`,
			wantError: true,
		},
		{
			name:      "invalid schema type value",
			schema:    `{"type": "not-a-valid-type"}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := domain.ValidateJSONSchema(tt.schema)
			if tt.wantError {
				require.Error(t, err)
				assert.True(t, domain.IsValidationError(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
