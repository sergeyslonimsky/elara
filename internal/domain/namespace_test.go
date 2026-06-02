package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestNamespace_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ns      domain.Namespace
		wantErr string
	}{
		{
			name: "valid namespace",
			ns: domain.Namespace{
				Name:        "default",
				DisplayName: "Default Namespace",
			},
		},
		{
			name: "invalid name",
			ns: domain.Namespace{
				Name: "Invalid Name",
			},
			wantErr: "validation: name",
		},
		{
			name: "display name too long",
			ns: domain.Namespace{
				Name:        "default",
				DisplayName: strings.Repeat("a", 129),
			},
			wantErr: "display name must be at most 128 characters",
		},
		{
			name: "display name at max length",
			ns: domain.Namespace{
				Name:        "default",
				DisplayName: strings.Repeat("a", 128),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.ns.Validate()

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.True(t, domain.IsValidationError(err))

				return
			}

			require.NoError(t, err)
		})
	}
}
