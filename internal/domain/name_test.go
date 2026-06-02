package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestValidateCanonicalName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		field   string
		value   string
		wantErr string
	}{
		{
			name:  "valid name",
			field: "name",
			value: "valid-name-123",
		},
		{
			name:    "empty name",
			field:   "name",
			value:   "",
			wantErr: "name is required",
		},
		{
			name:    "too long",
			field:   "name",
			value:   strings.Repeat("a", 64),
			wantErr: "name must be at most 63 characters",
		},
		{
			name:  "max length",
			field: "name",
			value: strings.Repeat("a", 63),
		},
		{
			name:    "invalid characters",
			field:   "name",
			value:   "Invalid_Name",
			wantErr: "name must be a valid DNS-1123 label",
		},
		{
			name:    "starts with hyphen",
			field:   "name",
			value:   "-invalid",
			wantErr: "name must be a valid DNS-1123 label",
		},
		{
			name:    "ends with hyphen",
			field:   "name",
			value:   "invalid-",
			wantErr: "name must be a valid DNS-1123 label",
		},
		{
			name:  "single character",
			field: "name",
			value: "a",
		},
		{
			name:  "numbers only",
			field: "name",
			value: "123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := domain.ValidateCanonicalName(tt.field, tt.value)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.True(t, domain.IsValidationError(err))

				return
			}

			require.NoError(t, err)
		})
	}
}
