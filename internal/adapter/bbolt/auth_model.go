package bbolt

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type authUserMeta struct {
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	Picture     string    `json:"picture"`
	Provider    string    `json:"provider"`
	CreatedAt   time.Time `json:"created_at"`
	LastLoginAt time.Time `json:"last_login_at"`
}

type authGroupMeta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Members   []string  `json:"members"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type authTokenMeta struct {
	ID         string     `json:"id"`
	UserEmail  string     `json:"user_email"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"token_hash"`
	Namespaces []string   `json:"namespaces"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	LastUsedIP string     `json:"last_used_ip"`
	CreatedAt  time.Time  `json:"created_at"`
}

func domainToAuthUserMeta(u *domain.User) *authUserMeta {
	return &authUserMeta{
		Email:       u.Email,
		Name:        u.Name,
		Picture:     u.Picture,
		Provider:    u.Provider,
		CreatedAt:   u.CreatedAt,
		LastLoginAt: u.LastLoginAt,
	}
}

func authUserMetaToDomain(m *authUserMeta) *domain.User {
	return &domain.User{
		Email:       m.Email,
		Name:        m.Name,
		Picture:     m.Picture,
		Provider:    m.Provider,
		CreatedAt:   m.CreatedAt,
		LastLoginAt: m.LastLoginAt,
	}
}

func domainToAuthGroupMeta(g *domain.Group) *authGroupMeta {
	members := make([]string, len(g.Members))
	copy(members, g.Members)

	return &authGroupMeta{
		ID:        g.ID,
		Name:      g.Name,
		Members:   members,
		CreatedAt: g.CreatedAt,
		UpdatedAt: g.UpdatedAt,
	}
}

func authGroupMetaToDomain(m *authGroupMeta) *domain.Group {
	members := make([]string, len(m.Members))
	copy(members, m.Members)

	return &domain.Group{
		ID:        m.ID,
		Name:      m.Name,
		Members:   members,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func domainToAuthTokenMeta(p *domain.PAT) *authTokenMeta {
	namespaces := make([]string, len(p.Namespaces))
	copy(namespaces, p.Namespaces)

	return &authTokenMeta{
		ID:         p.ID,
		UserEmail:  p.UserEmail,
		Name:       p.Name,
		TokenHash:  p.TokenHash,
		Namespaces: namespaces,
		ExpiresAt:  p.ExpiresAt,
		LastUsedAt: p.LastUsedAt,
		LastUsedIP: p.LastUsedIP,
		CreatedAt:  p.CreatedAt,
	}
}

func authTokenMetaToDomain(m *authTokenMeta) *domain.PAT {
	namespaces := make([]string, len(m.Namespaces))
	copy(namespaces, m.Namespaces)

	return &domain.PAT{
		ID:         m.ID,
		UserEmail:  m.UserEmail,
		Name:       m.Name,
		TokenHash:  m.TokenHash,
		Namespaces: namespaces,
		ExpiresAt:  m.ExpiresAt,
		LastUsedAt: m.LastUsedAt,
		LastUsedIP: m.LastUsedIP,
		CreatedAt:  m.CreatedAt,
	}
}

func authTokenMetaFromBytes(data []byte) (*authTokenMeta, error) {
	var token authTokenMeta

	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}

	return &token, nil
}

func authGroupMetaFromBytes(data []byte) (*authGroupMeta, error) {
	var group authGroupMeta

	if err := json.Unmarshal(data, &group); err != nil {
		return nil, fmt.Errorf("unmarshal group: %w", err)
	}

	return &group, nil
}
