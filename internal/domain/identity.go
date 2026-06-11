package domain

import (
	"fmt"
	"strings"
)

type Identity struct {
	Provider IdentityProvider `json:"provider"`
	Subject  string           `json:"subject"`
}

type IdentityProvider string

const (
	ProviderBasic IdentityProvider = "basic"
	// ProviderOIDC is the single-issuer shortcut. Multi-issuer setups should use OIDCProvider(issuerID).
	ProviderOIDC IdentityProvider = "oidc"

	oidcMultiIssuerPrefix = "oidc:"
)

// OIDCProvider builds the canonical provider tag for an OIDC issuer:
// "oidc:<issuer-id>". Returns IdentityProvider so call-sites can assign the
// result directly to Identity.Provider without a cast.
func OIDCProvider(issuerID string) IdentityProvider {
	return IdentityProvider(oidcMultiIssuerPrefix + issuerID)
}

// NewIdentity constructs an Identity, applying provider-specific
// normalization to rawSubject so that callers cannot accidentally persist
// non-canonical forms.
//
// Per-provider rules:
//
//   - ProviderBasic: Subject is the user's login handle, which Elara stores
//     and queries as a normalized email. rawSubject MUST satisfy
//     NormalizeEmail (exactly one '@', non-empty halves, ≤254 chars after
//     NFKC+lowercase). Any malformed input is rejected at construction.
//   - ProviderOIDC / "oidc:<issuer>": Subject is the IdP-issued opaque
//     `sub` claim. It is NOT touched — only required to be non-empty.
//
// Unknown providers return ErrInvalidIdentityProvider (programming bug, not
// user input). Empty rawSubject returns a ValidationError for any provider.
//
// This is the single chokepoint for Identity construction across the
// codebase — bootstrap, OIDC callback, user management. Going through
// NewIdentity guarantees that Identity.Subject is always in the same form
// as the lookup keys used by UserService.GetByIdentity, closing the
// case-sensitivity drift between bootstrap-stored and login-queried
// subjects.
func NewIdentity(provider IdentityProvider, rawSubject string) (Identity, error) {
	if rawSubject == "" {
		return Identity{}, NewValidationError("subject", "identity subject is required")
	}

	switch {
	case provider == ProviderBasic:
		normalized, err := NormalizeEmail(rawSubject)
		if err != nil {
			return Identity{}, fmt.Errorf("normalize basic subject: %w", err)
		}

		return Identity{Provider: provider, Subject: normalized}, nil

	case provider == ProviderOIDC || strings.HasPrefix(string(provider), oidcMultiIssuerPrefix):
		return Identity{Provider: provider, Subject: rawSubject}, nil

	default:
		return Identity{}, fmt.Errorf("provider %q: %w", provider, ErrInvalidIdentityProvider)
	}
}
