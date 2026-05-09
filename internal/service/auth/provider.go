package auth

import "context"

// Identity is the normalized user identity returned by any provider.
type Identity struct {
	Email   string
	Name    string
	Picture string
	Groups  []string // populated when the IdP supports a groups claim
}

// IdentityProvider is the abstraction for any OIDC-compatible IdP.
type IdentityProvider interface {
	Name() string
	AuthURL(state, nonce string) string
	Exchange(ctx context.Context, code, nonce string) (*Identity, error)
}
