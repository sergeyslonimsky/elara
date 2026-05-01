package domain

import (
	"strings"
	"time"
)

const providerOIDC = "oidc"

var validProviders = map[string]struct{}{
	providerOIDC: {},
}

type User struct {
	Email       string
	Name        string
	Picture     string
	Provider    string
	CreatedAt   time.Time
	LastLoginAt time.Time
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

	if _, ok := validProviders[u.Provider]; !ok {
		return NewValidationError("provider", "provider must be one of: oidc")
	}

	return nil
}
