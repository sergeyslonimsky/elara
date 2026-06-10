package internal

import (
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// TokenMeta is the on-disk JSON shape for a domain.Token. The primary
// bucket key is the SHA-256 hex TokenHash; a secondary index maps ID →
// TokenHash for O(1) lookups by ID.
type TokenMeta struct {
	ID         string     `json:"id"`
	IssuedBy   string     `json:"issued_by"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"token_hash"`
	Namespaces []string   `json:"namespaces"`
	Role       string     `json:"role"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	LastUsedIP string     `json:"last_used_ip"`
	CreatedAt  time.Time  `json:"created_at"`
}

func DomainToTokenMeta(t *domain.Token) TokenMeta {
	namespaces := make([]string, len(t.Namespaces))
	copy(namespaces, t.Namespaces)

	return TokenMeta{
		ID:         t.ID,
		IssuedBy:   t.IssuedBy,
		Name:       t.Name,
		TokenHash:  t.TokenHash,
		Namespaces: namespaces,
		Role:       string(t.Role),
		ExpiresAt:  t.ExpiresAt,
		LastUsedAt: t.LastUsedAt,
		LastUsedIP: t.LastUsedIP,
		CreatedAt:  t.CreatedAt,
	}
}

func TokenMetaToDomain(m TokenMeta) *domain.Token {
	namespaces := make([]string, len(m.Namespaces))
	copy(namespaces, m.Namespaces)

	return &domain.Token{
		ID:         m.ID,
		IssuedBy:   m.IssuedBy,
		Name:       m.Name,
		TokenHash:  m.TokenHash,
		Namespaces: namespaces,
		Role:       domain.Role(m.Role),
		ExpiresAt:  m.ExpiresAt,
		LastUsedAt: m.LastUsedAt,
		LastUsedIP: m.LastUsedIP,
		CreatedAt:  m.CreatedAt,
	}
}
