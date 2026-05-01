package domain

import (
	"strings"
	"time"
)

const maxPATNameLen = 128

// PAT represents a Personal Access Token used for API authentication.
type PAT struct {
	ID         string
	UserEmail  string
	Name       string
	TokenHash  string     // SHA-256 hex of raw token
	Namespaces []string   // empty = all namespaces
	ExpiresAt  *time.Time // nil = never expires
	LastUsedAt *time.Time
	LastUsedIP string
	CreatedAt  time.Time
}

func (p *PAT) Validate() error {
	if p.ID == "" {
		return NewValidationError("id", "id is required")
	}

	if p.UserEmail == "" {
		return NewValidationError("userEmail", "user email is required")
	}

	if !strings.Contains(p.UserEmail, "@") {
		return NewValidationError("userEmail", "user email must be a valid email address")
	}

	if p.Name == "" {
		return NewValidationError("name", "name is required")
	}

	if len(p.Name) > maxPATNameLen {
		return NewValidationError("name", "name must be at most 128 characters")
	}

	if p.TokenHash == "" {
		return NewValidationError("tokenHash", "token hash is required")
	}

	return nil
}

// IsExpired returns true if the token has a non-nil expiry that is in the past.
func (p *PAT) IsExpired() bool {
	return p.ExpiresAt != nil && p.ExpiresAt.Before(time.Now())
}

// HasNamespaceAccess returns true if the token grants access to the given namespace.
// An empty Namespaces slice means access to all namespaces.
func (p *PAT) HasNamespaceAccess(namespace string) bool {
	if len(p.Namespaces) == 0 {
		return true
	}

	for _, ns := range p.Namespaces {
		if ns == namespace {
			return true
		}
	}

	return false
}
