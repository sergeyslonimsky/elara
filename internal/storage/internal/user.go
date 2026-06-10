package internal

import (
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// AuthUserMeta is the on-disk JSON shape for a domain.User. The bucket key
// is the user ID (UUID string); ID is also stored in the value for forward
// compatibility with index-only lookups.
type AuthUserMeta struct {
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

func DomainToAuthUserMeta(u *domain.User) AuthUserMeta {
	return AuthUserMeta{
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

func AuthUserMetaToDomain(m AuthUserMeta) *domain.User {
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
