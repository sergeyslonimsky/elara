package bbolt

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type authUserMeta struct {
	ID                     string            `json:"id"`
	Email                  string            `json:"email"`
	DisplayName            string            `json:"display_name,omitempty"`
	Picture                string            `json:"picture,omitempty"`
	Status                 domain.UserStatus `json:"status"`
	Identities             []domain.Identity `json:"identities,omitempty"`
	System                 bool              `json:"system,omitempty"`
	CreatedAt              time.Time         `json:"created_at"`
	LastLoginAt            time.Time         `json:"last_login_at"`
	PasswordHash           string            `json:"password_hash,omitempty"`
	PasswordChangeRequired bool              `json:"password_change_required,omitempty"`
	MembershipVersion      int64             `json:"membership_version,omitempty"`
}

// authGroupMeta is the bbolt JSON shape for a group entity.
//
// Membership (user→group) and permissions live exclusively in Casbin
// (g-rules / p-rules); this struct only carries the entity metadata
// bbolt is authoritative for. The three version counters track
// optimistic-lock state independently per editable slot.
type authGroupMeta struct {
	Name               string    `json:"name"`
	DisplayName        string    `json:"display_name,omitempty"`
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
		ID:                     u.ID.String(),
		Email:                  u.Email,
		DisplayName:            u.DisplayName,
		Picture:                u.Picture,
		Status:                 u.Status,
		Identities:             u.Identities,
		System:                 u.System,
		CreatedAt:              u.CreatedAt,
		LastLoginAt:            u.LastLoginAt,
		PasswordHash:           u.PasswordHash,
		PasswordChangeRequired: u.PasswordChangeRequired,
		MembershipVersion:      u.MembershipVersion,
	}
}

func authUserMetaToDomain(m *authUserMeta) *domain.User {
	id, _ := uuid.Parse(m.ID)

	return &domain.User{
		ID:                     id,
		Email:                  m.Email,
		DisplayName:            m.DisplayName,
		Picture:                m.Picture,
		Status:                 m.Status,
		Identities:             m.Identities,
		System:                 m.System,
		CreatedAt:              m.CreatedAt,
		LastLoginAt:            m.LastLoginAt,
		PasswordHash:           m.PasswordHash,
		PasswordChangeRequired: m.PasswordChangeRequired,
		MembershipVersion:      m.MembershipVersion,
	}
}

func domainToAuthGroupMeta(g *domain.Group) *authGroupMeta {
	return &authGroupMeta{
		Name:               g.Name,
		DisplayName:        g.DisplayName,
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
		Name:               m.Name,
		DisplayName:        m.DisplayName,
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
