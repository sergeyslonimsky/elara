package domain

type Identity struct {
	Provider IdentityProvider `json:"provider"`
	Subject  string           `json:"subject"`
}

type IdentityProvider string

const (
	ProviderBasic IdentityProvider = "basic"
	// ProviderOIDC is the single-issuer shortcut. Multi-issuer setups should use OIDCProvider(issuerID).
	ProviderOIDC IdentityProvider = "oidc"
)

// OIDCProvider builds the canonical provider tag for an OIDC issuer:
// "oidc:<issuer-id>". Returns IdentityProvider so call-sites can assign the
// result directly to Identity.Provider without a cast.
func OIDCProvider(issuerID string) IdentityProvider {
	return IdentityProvider("oidc:" + issuerID)
}
