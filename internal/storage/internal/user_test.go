package internal_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storageinternal "github.com/sergeyslonimsky/elara/internal/storage/internal"
)

func TestDomainToAuthUserMeta(t *testing.T) {
	t.Parallel()

	now := time.Now()
	id := uuid.New()

	tests := []struct {
		name string
		user *domain.User
		want storageinternal.AuthUserMeta
	}{
		{
			name: "full user",
			user: &domain.User{
				ID:          id,
				Email:       "user@example.com",
				DisplayName: "User",
				Picture:     "pic.png",
				Status:      domain.UserStatusActive,
				Identities: []domain.Identity{
					{Provider: domain.ProviderBasic, Subject: "user@example.com"},
				},
				System:                 true,
				CreatedAt:              now,
				LastLoginAt:            now,
				PasswordHash:           "hash",
				PasswordChangeRequired: true,
				MembershipVersion:      3,
			},
			want: storageinternal.AuthUserMeta{
				ID:          id.String(),
				Email:       "user@example.com",
				DisplayName: "User",
				Picture:     "pic.png",
				Status:      domain.UserStatusActive,
				Identities: []domain.Identity{
					{Provider: domain.ProviderBasic, Subject: "user@example.com"},
				},
				System:                 true,
				CreatedAt:              now,
				LastLoginAt:            now,
				PasswordHash:           "hash",
				PasswordChangeRequired: true,
				MembershipVersion:      3,
			},
		},
		{
			name: "zero value user",
			user: &domain.User{},
			want: storageinternal.AuthUserMeta{
				ID: uuid.Nil.String(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.DomainToAuthUserMeta(tt.user)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAuthUserMetaToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now()
	id := uuid.New()

	tests := []struct {
		name string
		meta storageinternal.AuthUserMeta
		want *domain.User
	}{
		{
			name: "valid uuid",
			meta: storageinternal.AuthUserMeta{
				ID:          id.String(),
				Email:       "user@example.com",
				DisplayName: "User",
				Picture:     "pic.png",
				Status:      domain.UserStatusActive,
				Identities: []domain.Identity{
					{Provider: domain.ProviderOIDC, Subject: "sub-1"},
				},
				System:                 false,
				CreatedAt:              now,
				LastLoginAt:            now,
				PasswordHash:           "hash",
				PasswordChangeRequired: false,
				MembershipVersion:      7,
			},
			want: &domain.User{
				ID:          id,
				Email:       "user@example.com",
				DisplayName: "User",
				Picture:     "pic.png",
				Status:      domain.UserStatusActive,
				Identities: []domain.Identity{
					{Provider: domain.ProviderOIDC, Subject: "sub-1"},
				},
				CreatedAt:         now,
				LastLoginAt:       now,
				PasswordHash:      "hash",
				MembershipVersion: 7,
			},
		},
		{
			name: "invalid uuid falls back to Nil",
			meta: storageinternal.AuthUserMeta{
				ID: "not-a-uuid",
			},
			want: &domain.User{
				ID: uuid.Nil,
			},
		},
		{
			name: "empty id falls back to Nil",
			meta: storageinternal.AuthUserMeta{},
			want: &domain.User{
				ID: uuid.Nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.AuthUserMetaToDomain(tt.meta)
			assert.Equal(t, tt.want, got)
		})
	}
}
