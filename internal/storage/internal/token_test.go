package internal_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storageinternal "github.com/sergeyslonimsky/elara/internal/storage/internal"
)

func TestDomainToTokenMeta(t *testing.T) {
	t.Parallel()

	now := time.Now()
	expiresAt := now.Add(time.Hour)
	lastUsedAt := now.Add(time.Minute)

	tests := []struct {
		name  string
		token *domain.Token
		want  storageinternal.TokenMeta
	}{
		{
			name: "full token",
			token: &domain.Token{
				ID:         "tok-1",
				IssuedBy:   "user@example.com",
				Name:       "my token",
				TokenHash:  "hash",
				Namespaces: []string{"default", "prod"},
				Role:       domain.RoleWriter,
				ExpiresAt:  &expiresAt,
				LastUsedAt: &lastUsedAt,
				LastUsedIP: "1.2.3.4",
				CreatedAt:  now,
			},
			want: storageinternal.TokenMeta{
				ID:         "tok-1",
				IssuedBy:   "user@example.com",
				Name:       "my token",
				TokenHash:  "hash",
				Namespaces: []string{"default", "prod"},
				Role:       "writer",
				ExpiresAt:  &expiresAt,
				LastUsedAt: &lastUsedAt,
				LastUsedIP: "1.2.3.4",
				CreatedAt:  now,
			},
		},
		{
			name: "nil namespaces becomes empty slice",
			token: &domain.Token{
				ID: "tok-2",
			},
			want: storageinternal.TokenMeta{
				ID:         "tok-2",
				Namespaces: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.DomainToTokenMeta(tt.token)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTokenMetaToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now()
	expiresAt := now.Add(time.Hour)
	lastUsedAt := now.Add(time.Minute)

	tests := []struct {
		name string
		meta storageinternal.TokenMeta
		want *domain.Token
	}{
		{
			name: "full meta",
			meta: storageinternal.TokenMeta{
				ID:         "tok-1",
				IssuedBy:   "user@example.com",
				Name:       "my token",
				TokenHash:  "hash",
				Namespaces: []string{"default", "prod"},
				Role:       "reader",
				ExpiresAt:  &expiresAt,
				LastUsedAt: &lastUsedAt,
				LastUsedIP: "1.2.3.4",
				CreatedAt:  now,
			},
			want: &domain.Token{
				ID:         "tok-1",
				IssuedBy:   "user@example.com",
				Name:       "my token",
				TokenHash:  "hash",
				Namespaces: []string{"default", "prod"},
				Role:       domain.RoleReader,
				ExpiresAt:  &expiresAt,
				LastUsedAt: &lastUsedAt,
				LastUsedIP: "1.2.3.4",
				CreatedAt:  now,
			},
		},
		{
			name: "nil namespaces becomes empty slice",
			meta: storageinternal.TokenMeta{
				ID: "tok-2",
			},
			want: &domain.Token{
				ID:         "tok-2",
				Namespaces: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.TokenMetaToDomain(tt.meta)
			assert.Equal(t, tt.want, got)
		})
	}
}
