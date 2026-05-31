package bbolt

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type authUserMeta struct {
	Email                  string    `json:"email"`
	Name                   string    `json:"name"`
	Picture                string    `json:"picture"`
	Provider               string    `json:"provider"`
	System                 bool      `json:"system,omitempty"`
	Source                 string    `json:"source,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	LastLoginAt            time.Time `json:"last_login_at"`
	PasswordHash           string    `json:"password_hash,omitempty"`
	PasswordChangeRequired bool      `json:"password_change_required,omitempty"`
	MembershipVersion      int64     `json:"membership_version,omitempty"`
}

// authGroupMeta is the bbolt JSON shape for a group entity.
//
// Membership (user→group) and permissions live exclusively in Casbin
// (g-rules / p-rules); this struct only carries the entity metadata
// bbolt is authoritative for. The three version counters track
// optimistic-lock state independently per editable slot.
type authGroupMeta struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description,omitempty"`
	System             bool      `json:"system,omitempty"`
	MetadataVersion    int64     `json:"metadata_version,omitempty"`
	MembersVersion     int64     `json:"members_version,omitempty"`
	PermissionsVersion int64     `json:"permissions_version,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type authTokenMeta struct {
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

func domainToAuthUserMeta(u *domain.User) *authUserMeta {
	return &authUserMeta{
		Email:                  u.Email,
		Name:                   u.Name,
		Picture:                u.Picture,
		Provider:               u.Provider,
		System:                 u.System,
		Source:                 u.Source,
		CreatedAt:              u.CreatedAt,
		LastLoginAt:            u.LastLoginAt,
		PasswordHash:           u.PasswordHash,
		PasswordChangeRequired: u.PasswordChangeRequired,
		MembershipVersion:      u.MembershipVersion,
	}
}

func authUserMetaToDomain(m *authUserMeta) *domain.User {
	return &domain.User{
		Email:                  m.Email,
		Name:                   m.Name,
		Picture:                m.Picture,
		Provider:               m.Provider,
		System:                 m.System,
		Source:                 m.Source,
		CreatedAt:              m.CreatedAt,
		LastLoginAt:            m.LastLoginAt,
		PasswordHash:           m.PasswordHash,
		PasswordChangeRequired: m.PasswordChangeRequired,
		MembershipVersion:      m.MembershipVersion,
	}
}

func domainToAuthGroupMeta(g *domain.Group) *authGroupMeta {
	return &authGroupMeta{
		ID:                 g.ID,
		Name:               g.Name,
		Description:        g.Description,
		System:             g.System,
		MetadataVersion:    g.MetadataVersion,
		MembersVersion:     g.MembersVersion,
		PermissionsVersion: g.PermissionsVersion,
		CreatedAt:          g.CreatedAt,
		UpdatedAt:          g.UpdatedAt,
	}
}

func authGroupMetaToDomain(m *authGroupMeta) *domain.Group {
	return &domain.Group{
		ID:                 m.ID,
		Name:               m.Name,
		Description:        m.Description,
		System:             m.System,
		MetadataVersion:    m.MetadataVersion,
		MembersVersion:     m.MembersVersion,
		PermissionsVersion: m.PermissionsVersion,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}

func domainToAuthTokenMeta(t *domain.Token) *authTokenMeta {
	namespaces := make([]string, len(t.Namespaces))
	copy(namespaces, t.Namespaces)

	return &authTokenMeta{
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

func authTokenMetaToDomain(m *authTokenMeta) *domain.Token {
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
