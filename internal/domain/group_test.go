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
		Members:   []string{"alice@example.com", "bob@example.com"},
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
			name: "invalid member email no at sign",
			group: domain.Group{
				ID:      "group-1",
				Name:    "Admins",
				Members: []string{"alice@example.com", "notanemail"},
			},
			wantErr: true,
			errMsg:  "members",
		},
		{
			name: "empty member email",
			group: domain.Group{
				ID:      "group-1",
				Name:    "Admins",
				Members: []string{""},
			},
			wantErr: true,
			errMsg:  "members",
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

func TestGroup_AddMember(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		initial     []string
		addEmail    string
		wantErr     bool
		wantErrIs   error
		wantMembers []string
	}{
		{
			name:        "adds member successfully",
			initial:     []string{"alice@example.com"},
			addEmail:    "bob@example.com",
			wantErr:     false,
			wantMembers: []string{"alice@example.com", "bob@example.com"},
		},
		{
			name:      "duplicate member returns ErrAlreadyExists",
			initial:   []string{"alice@example.com"},
			addEmail:  "alice@example.com",
			wantErr:   true,
			wantErrIs: domain.ErrAlreadyExists,
		},
		{
			name:      "invalid email returns validation error",
			initial:   []string{},
			addEmail:  "notanemail",
			wantErr:   true,
			wantErrIs: nil, // ValidationError, checked separately
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := domain.Group{
				ID:      "group-1",
				Name:    "Test",
				Members: append([]string(nil), tt.initial...),
			}

			err := g.AddMember(tt.addEmail)

			if tt.wantErr {
				require.Error(t, err)

				if tt.wantErrIs != nil {
					require.ErrorIs(t, err, tt.wantErrIs)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantMembers, g.Members)
			}
		})
	}
}

func TestGroup_RemoveMember(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		initial     []string
		removeEmail string
		wantErr     bool
		wantErrIs   error
		wantMembers []string
	}{
		{
			name:        "removes member successfully",
			initial:     []string{"alice@example.com", "bob@example.com"},
			removeEmail: "alice@example.com",
			wantErr:     false,
			wantMembers: []string{"bob@example.com"},
		},
		{
			name:        "not found returns ErrNotFound",
			initial:     []string{"alice@example.com"},
			removeEmail: "charlie@example.com",
			wantErr:     true,
			wantErrIs:   domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := domain.Group{
				ID:      "group-1",
				Name:    "Test",
				Members: append([]string(nil), tt.initial...),
			}

			err := g.RemoveMember(tt.removeEmail)

			if tt.wantErr {
				require.Error(t, err)

				if tt.wantErrIs != nil {
					require.ErrorIs(t, err, tt.wantErrIs)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantMembers, g.Members)
			}
		})
	}
}
