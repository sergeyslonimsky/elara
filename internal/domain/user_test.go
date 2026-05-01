package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestUser_Validate(t *testing.T) {
	t.Parallel()

	validUser := domain.User{
		Email:       "alice@example.com",
		Name:        "Alice",
		Provider:    "oidc",
		CreatedAt:   time.Now(),
		LastLoginAt: time.Now(),
	}

	tests := []struct {
		name    string
		user    domain.User
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid user",
			user:    validUser,
			wantErr: false,
		},
		{
			name: "empty email",
			user: domain.User{
				Email:    "",
				Name:     "Alice",
				Provider: "oidc",
			},
			wantErr: true,
			errMsg:  "email",
		},
		{
			name: "invalid email no at sign",
			user: domain.User{
				Email:    "notanemail",
				Name:     "Alice",
				Provider: "oidc",
			},
			wantErr: true,
			errMsg:  "email",
		},
		{
			name: "empty name",
			user: domain.User{
				Email:    "alice@example.com",
				Name:     "",
				Provider: "oidc",
			},
			wantErr: true,
			errMsg:  "name",
		},
		{
			name: "invalid provider",
			user: domain.User{
				Email:    "alice@example.com",
				Name:     "Alice",
				Provider: "github",
			},
			wantErr: true,
			errMsg:  "provider",
		},
		{
			name: "empty provider",
			user: domain.User{
				Email:    "alice@example.com",
				Name:     "Alice",
				Provider: "",
			},
			wantErr: true,
			errMsg:  "provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.user.Validate()

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
