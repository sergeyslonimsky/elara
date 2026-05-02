package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func validPAT() domain.PAT {
	return domain.PAT{
		ID:        "pat-1",
		UserEmail: "alice@example.com",
		Name:      "My Token",
		TokenHash: "abc123def456",
		CreatedAt: time.Now(),
	}
}

func TestPAT_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pat     domain.PAT
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid PAT",
			pat:     validPAT(),
			wantErr: false,
		},
		{
			name: "empty ID",
			pat: func() domain.PAT {
				p := validPAT()
				p.ID = ""

				return p
			}(),
			wantErr: true,
			errMsg:  "id",
		},
		{
			name: "empty user email",
			pat: func() domain.PAT {
				p := validPAT()
				p.UserEmail = ""

				return p
			}(),
			wantErr: true,
			errMsg:  "userEmail",
		},
		{
			name: "invalid user email no at sign",
			pat: func() domain.PAT {
				p := validPAT()
				p.UserEmail = "notanemail"

				return p
			}(),
			wantErr: true,
			errMsg:  "userEmail",
		},
		{
			name: "empty name",
			pat: func() domain.PAT {
				p := validPAT()
				p.Name = ""

				return p
			}(),
			wantErr: true,
			errMsg:  "name",
		},
		{
			name: "name too long",
			pat: func() domain.PAT {
				p := validPAT()
				p.Name = strings.Repeat("a", 129)

				return p
			}(),
			wantErr: true,
			errMsg:  "name",
		},
		{
			name: "name at max length is valid",
			pat: func() domain.PAT {
				p := validPAT()
				p.Name = strings.Repeat("a", 128)

				return p
			}(),
			wantErr: false,
		},
		{
			name: "empty token hash",
			pat: func() domain.PAT {
				p := validPAT()
				p.TokenHash = ""

				return p
			}(),
			wantErr: true,
			errMsg:  "tokenHash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.pat.Validate()

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

func TestPAT_IsExpired(t *testing.T) {
	t.Parallel()

	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{
			name:      "nil expiry is never expired",
			expiresAt: nil,
			want:      false,
		},
		{
			name:      "past expiry is expired",
			expiresAt: &past,
			want:      true,
		},
		{
			name:      "future expiry is not expired",
			expiresAt: &future,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := validPAT()
			p.ExpiresAt = tt.expiresAt

			assert.Equal(t, tt.want, p.IsExpired())
		})
	}
}

func TestPAT_HasNamespaceAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		namespaces []string
		namespace  string
		want       bool
	}{
		{
			name:       "empty namespaces grants access to all",
			namespaces: []string{},
			namespace:  "production",
			want:       true,
		},
		{
			name:       "nil namespaces grants access to all",
			namespaces: nil,
			namespace:  "production",
			want:       true,
		},
		{
			name:       "matching namespace grants access",
			namespaces: []string{"staging", "production"},
			namespace:  "production",
			want:       true,
		},
		{
			name:       "non-matching namespace denies access",
			namespaces: []string{"staging"},
			namespace:  "production",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := validPAT()
			p.Namespaces = tt.namespaces

			assert.Equal(t, tt.want, p.HasNamespaceAccess(tt.namespace))
		})
	}
}
