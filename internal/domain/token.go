package domain

import (
	"slices"
	"strings"
	"time"
)

// TokenFilter narrows a token list to those whose namespace scope intersects
// the caller's allowed namespaces.
//
// When AnyNamespace is true, NamespaceScopes MUST be ignored. Tokens that
// expose at least one namespace from NamespaceScopes match; tokens whose
// namespaces all fall outside the set are filtered out.
//
// IssuedBy is an additional narrowing (admin / UI may want a particular
// user's tokens); a token matches when its issuer is in the list, or the
// list is empty.
//
// Namespaces narrows to tokens that explicitly grant access to at least one
// of the listed namespaces (independent of NamespaceScopes which enforces
// the caller's visibility). Empty list disables this filter.
//
// QueryParams applies case-insensitive substring matches on Name. A token
// matches when its name contains any of the entries; empty list disables it.
type TokenFilter struct {
	NamespaceScopes map[string]struct{}
	AnyNamespace    bool
	IssuedBy        []string
	Namespaces      []string
	QueryParams     []string
}

// TokenListParams carries pagination and sort options for token list queries.
type TokenListParams struct {
	Limit  int
	Offset int
	Sort   SortParams
}

// Token is a service credential issued by a user for use by external etcd clients.
//
// Name is the prefix the issuer chose; Username is Name plus a
// server-generated suffix and is the globally unique login an etcd client
// presents (`etcdctl --user <Username>:<raw token>`). Names may repeat across
// tokens; Usernames never do, so a user can't learn of tokens they can't see
// by picking a name that collides.
type Token struct {
	ID         string
	IssuedBy   string // email of the user who created this token
	Name       string
	Username   string     // immutable, unique; Name + "-" + random suffix
	TokenHash  string     // SHA-256 hex of raw token
	Namespaces []string   // explicit list; must be non-empty
	Role       Role       // "writer" or "reader"
	ExpiresAt  *time.Time // nil = never expires
	LastUsedAt *time.Time
	LastUsedIP string
	CreatedAt  time.Time
}

func (t *Token) Validate() error {
	if t.ID == "" {
		return NewValidationError("id", "id is required")
	}

	if t.IssuedBy == "" {
		return NewValidationError("issuedBy", "issued_by is required")
	}

	if !strings.Contains(t.IssuedBy, "@") {
		return NewValidationError("issuedBy", "issued_by must be a valid email address")
	}

	if err := ValidateTokenNamePrefix(t.Name); err != nil {
		return err
	}

	if t.Username == "" {
		return NewValidationError("username", "username is required")
	}

	if t.TokenHash == "" {
		return NewValidationError("tokenHash", "token hash is required")
	}

	if t.Role != RoleWriter && t.Role != RoleReader {
		return NewValidationError("role", "role must be writer or reader")
	}

	if len(t.Namespaces) == 0 {
		return NewValidationError("namespaces", "at least one namespace is required")
	}

	return nil
}

// IsExpired returns true if the token has a non-nil expiry that is in the past.
func (t *Token) IsExpired() bool {
	return t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now())
}

// NamespaceAllowed returns true if the token grants access to the given namespace.
func (t *Token) NamespaceAllowed(namespace string) bool {
	return slices.Contains(t.Namespaces, namespace)
}

// ActionAllowed returns true if the token's role permits the given action.
func (t *Token) ActionAllowed(action Action) bool {
	switch t.Role {
	case RoleWriter:
		return action == ActionRead || action == ActionWrite
	case RoleReader:
		return action == ActionRead
	case RoleAdmin:
		return false
	}

	return false
}
