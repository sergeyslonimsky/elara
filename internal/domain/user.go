package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AuthType string

const (
	AuthTypeOIDC      AuthType = "oidc"
	AuthTypeBasicAuth AuthType = "basic-auth"
	AuthTypeNone      AuthType = "none"
)

// UserFilter narrows a user list to those the caller is permitted to see.
//
// When AnyUser is true, UserIDs MUST be ignored — every user matches. This
// represents the wildcard scope (caller can read every group, hence every
// user). Search applies a case-insensitive substring match on Email and
// is independent of AnyUser/UserIDs.
type UserFilter struct {
	UserIDs map[string]struct{}
	AnyUser bool
	Search  string
}

// UserListParams carries pagination and sort options for user list queries.
type UserListParams struct {
	Limit  int
	Offset int
	Sort   SortParams
}

type User struct {
	ID                     uuid.UUID
	Email                  string // login hint; required, globally unique (see EL-50 §3.3)
	DisplayName            string
	Picture                string
	Status                 UserStatus
	Identities             []Identity
	System                 bool // protected from delete/rename; set by Seed, never by the API
	CreatedAt              time.Time
	LastLoginAt            time.Time
	PasswordHash           string
	PasswordChangeRequired bool
	// Optimistic-lock counter for group memberships. Bumped on every
	// UpdateUserGroups apply. Bbolt is authoritative — the value reflects
	// the last successful mutation through the membership usecase.
	MembershipVersion int64
}

func (u *User) EnsureMutable() error {
	if u.System {
		return ErrSystemImmutable
	}

	return nil
}

func (u *User) Validate() error {
	if u.ID == uuid.Nil {
		return NewValidationError("id", "user id is required")
	}

	if u.Email == "" {
		return NewValidationError("email", "email is required")
	}

	// Email shape (exactly one @, non-empty halves, NFKC, ≤254 chars) is
	// enforced by NormalizeEmail at the service boundary — domain Validate
	// only checks the post-normalization invariant of "non-empty".

	if len(u.DisplayName) > maxDisplayNameLen {
		return NewValidationError(
			"displayName",
			fmt.Sprintf("display name must be at most %d characters", maxDisplayNameLen),
		)
	}

	if !u.Status.Valid() {
		return NewValidationError("status", "invalid user status")
	}

	return nil
}

func (u *User) Deactivate() error {
	if err := u.EnsureMutable(); err != nil {
		return err
	}
	if u.Status == UserStatusDeactivated {
		return NewValidationError("status", "user is already deactivated")
	}
	u.Status = UserStatusDeactivated

	return nil
}

func (u *User) Reactivate() error {
	if err := u.EnsureMutable(); err != nil {
		return err
	}
	if u.Status == UserStatusActive {
		return NewValidationError("status", "user is already active")
	}
	u.Status = UserStatusActive

	return nil
}
