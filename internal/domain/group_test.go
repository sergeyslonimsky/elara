package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestGroup_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   domain.Group
		wantErr string
	}{
		{
			name: "valid group",
			group: domain.Group{
				Name:        "admins",
				DisplayName: "Administrators",
				Description: "System administrators",
			},
		},
		{
			name: "empty name",
			group: domain.Group{
				Name: "",
			},
			wantErr: "name is required",
		},
		{
			name: "name too long",
			group: domain.Group{
				Name: strings.Repeat("a", 64),
			},
			wantErr: "name must be at most 63 characters",
		},
		{
			name: "name at max length is valid",
			group: domain.Group{
				Name: strings.Repeat("a", 63),
			},
		},
		{
			name: "uppercase in name is invalid",
			group: domain.Group{
				Name: "Admins",
			},
			wantErr: "name must be a valid DNS-1123 label",
		},
		{
			name: "display name too long",
			group: domain.Group{
				Name:        "admins",
				DisplayName: strings.Repeat("a", 129),
			},
			wantErr: "display name must be at most 128 characters",
		},
		{
			name: "description too long",
			group: domain.Group{
				Name:        "admins",
				Description: strings.Repeat("a", 1025),
			},
			wantErr: "group description must be at most 1024 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.group.Validate()

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.True(t, domain.IsValidationError(err))

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestGroup_EnsureMutable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		group domain.Group
		errIs error
	}{
		{
			name:  "mutable group",
			group: domain.Group{System: false},
		},
		{
			name:  "immutable system group",
			group: domain.Group{System: true},
			errIs: domain.ErrSystemImmutable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.group.EnsureMutable()

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			require.NoError(t, err)
		})
	}
}
