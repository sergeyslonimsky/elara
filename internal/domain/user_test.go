package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestUser_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		user    domain.User
		wantErr string
	}{
		{
			name: "valid user",
			user: domain.User{
				ID:          uuid.New(),
				Email:       "alice@example.com",
				DisplayName: "Alice",
				Status:      domain.UserStatusActive,
				CreatedAt:   time.Now(),
				LastLoginAt: time.Now(),
			},
		},
		{
			name: "empty ID",
			user: domain.User{
				Email:  "alice@example.com",
				Status: domain.UserStatusActive,
			},
			wantErr: "user id is required",
		},
		{
			name: "empty email",
			user: domain.User{
				ID:     uuid.New(),
				Email:  "",
				Status: domain.UserStatusActive,
			},
			wantErr: "email is required",
		},
		{
			name: "display name too long",
			user: domain.User{
				ID:          uuid.New(),
				Email:       "alice@example.com",
				DisplayName: strings.Repeat("a", 129),
				Status:      domain.UserStatusActive,
			},
			wantErr: "display name must be at most 128 characters",
		},
		{
			name: "invalid status",
			user: domain.User{
				ID:     uuid.New(),
				Email:  "alice@example.com",
				Status: "invalid",
			},
			wantErr: "invalid user status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.user.Validate()

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.True(t, domain.IsValidationError(err))

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestUser_EnsureMutable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		user  domain.User
		errIs error
	}{
		{
			name: "mutable user",
			user: domain.User{System: false},
		},
		{
			name:  "immutable system user",
			user:  domain.User{System: true},
			errIs: domain.ErrSystemImmutable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.user.EnsureMutable()

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestOIDCProvider(t *testing.T) {
	t.Parallel()

	assert.Equal(t, domain.IdentityProvider("oidc:google"), domain.OIDCProvider("google"))
}

func TestUserStatus_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status domain.UserStatus
		want   bool
	}{
		{domain.UserStatusActive, true},
		{domain.UserStatusDeactivated, true},
		{domain.UserStatus("invalid"), false},
		{domain.UserStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.status.Valid())
		})
	}
}

func TestUser_DeactivateReactivate(t *testing.T) {
	t.Parallel()

	t.Run("deactivate mutable active user", func(t *testing.T) {
		t.Parallel()
		u := domain.User{System: false, Status: domain.UserStatusActive}
		err := u.Deactivate()
		require.NoError(t, err)
		assert.Equal(t, domain.UserStatusDeactivated, u.Status)
	})

	t.Run("deactivate system user rejected", func(t *testing.T) {
		t.Parallel()
		u := domain.User{System: true, Status: domain.UserStatusActive}
		err := u.Deactivate()
		require.ErrorIs(t, err, domain.ErrSystemImmutable)
	})

	t.Run("deactivate already deactivated user rejected", func(t *testing.T) {
		t.Parallel()
		u := domain.User{System: false, Status: domain.UserStatusDeactivated}
		err := u.Deactivate()
		require.Error(t, err)
		assert.True(t, domain.IsValidationError(err))
	})

	t.Run("reactivate mutable deactivated user", func(t *testing.T) {
		t.Parallel()
		u := domain.User{System: false, Status: domain.UserStatusDeactivated}
		err := u.Reactivate()
		require.NoError(t, err)
		assert.Equal(t, domain.UserStatusActive, u.Status)
	})

	t.Run("reactivate system user rejected", func(t *testing.T) {
		t.Parallel()
		u := domain.User{System: true, Status: domain.UserStatusDeactivated}
		err := u.Reactivate()
		require.ErrorIs(t, err, domain.ErrSystemImmutable)
	})

	t.Run("reactivate already active user rejected", func(t *testing.T) {
		t.Parallel()
		u := domain.User{System: false, Status: domain.UserStatusActive}
		err := u.Reactivate()
		require.Error(t, err)
		assert.True(t, domain.IsValidationError(err))
	})
}
