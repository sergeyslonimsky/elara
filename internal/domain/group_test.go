package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestGroup_Validate(t *testing.T) {
	t.Parallel()

	validGroup := domain.Group{
		ID:        "group-1",
		Name:      "Admins",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	tests := []struct {
		name    string
		group   domain.Group
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid group",
			group:   validGroup,
			wantErr: false,
		},
		{
			name: "empty ID",
			group: domain.Group{
				ID:   "",
				Name: "Admins",
			},
			wantErr: true,
			errMsg:  "id",
		},
		{
			name: "empty name",
			group: domain.Group{
				ID:   "group-1",
				Name: "",
			},
			wantErr: true,
			errMsg:  "name",
		},
		{
			name: "name too long",
			group: domain.Group{
				ID:   "group-1",
				Name: strings.Repeat("a", 129),
			},
			wantErr: true,
			errMsg:  "name",
		},
		{
			name: "name at max length is valid",
			group: domain.Group{
				ID:   "group-1",
				Name: strings.Repeat("a", 128),
			},
			wantErr: false,
		},
		{
			name: "no members is valid",
			group: domain.Group{
				ID:   "group-1",
				Name: "Admins",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.group.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.True(t, domain.IsValidationError(err))

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGroup_EnsureMutable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   domain.Group
		wantErr bool
		errIs   error
	}{
		{
			name:    "mutable group",
			group:   domain.Group{System: false},
			wantErr: false,
		},
		{
			name:    "immutable system group",
			group:   domain.Group{System: true},
			wantErr: true,
			errIs:   domain.ErrSystemImmutable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.group.EnsureMutable()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errIs != nil {
					require.ErrorIs(t, err, tt.errIs)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
