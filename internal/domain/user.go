package domain

import (
	"strings"
	"time"
)

const (
	ProviderOIDC      = "oidc"
	ProviderBasicAuth = "basic-auth"
)

// UserFilter narrows a user list to those the caller is permitted to see.
//
// When AnyUser is true, Usernames MUST be ignored — every user matches. This
// represents the wildcard scope (caller can read every group, hence every
// user). Search applies a case-insensitive substring match on Email/Name and
// is independent of AnyUser/Usernames.
type UserFilter struct {
	Usernames map[string]struct{}
	AnyUser   bool
	Search    string
}

// UserListParams carries pagination and sort options for user list queries.
type UserListParams struct {
	Limit  int
	Offset int
	Sort   SortParams
}

// Source identifies where a user was created. Kept as a typed string so OIDC /
// admin / seed handlers cannot drift on casing.
const (
	SourceLocal = "local"
	SourceOIDC  = "oidc"
	SourceSeed  = "seed"
)

type User struct {
	Email                  string
	Name                   string
	Picture                string
	Provider               string
	System                 bool   // protected from delete/rename; set by Seed, never by the API
	Source                 string // where the user came from (e.g. "seed", "oidc", "admin")
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
	if u.Email == "" {
		return NewValidationError("email", "email is required")
	}

	if !strings.Contains(u.Email, "@") {
		return NewValidationError("email", "email must be a valid email address")
	}

	if u.Name == "" {
		return NewValidationError("name", "name is required")
	}

	validProviders := map[string]struct{}{
		ProviderOIDC:      {},
		ProviderBasicAuth: {},
	}
	if _, ok := validProviders[u.Provider]; !ok {
		return NewValidationError("provider", "provider must be one of: oidc, basic-auth")
	}

	return nil
}
