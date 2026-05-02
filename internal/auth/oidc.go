package auth

import (
	"context"
	"errors"
	"fmt"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var (
	errIDTokenMissing = errors.New("id_token missing from response")
	errNonceMismatch  = errors.New("nonce mismatch")
)

// OIDCConfig holds the configuration required to set up an OIDC identity provider.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string // default: ["openid", "email", "profile"]
}

// OIDCProvider implements IdentityProvider using the go-oidc/v3 library.
type OIDCProvider struct {
	name     string
	verifier *gooidc.IDTokenVerifier
	oauth2   oauth2.Config
}

// NewOIDCProvider discovers the OIDC configuration from the issuer and returns a ready-to-use provider.
func NewOIDCProvider(ctx context.Context, cfg OIDCConfig) (*OIDCProvider, error) {
	provider, err := gooidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider %q: %w", cfg.IssuerURL, err)
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{gooidc.ScopeOpenID, "email", "profile"}
	}

	return &OIDCProvider{
		name:     "oidc",
		verifier: provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID}),
		oauth2: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       scopes,
		},
	}, nil
}

// Name returns the provider identifier.
func (p *OIDCProvider) Name() string {
	return p.name
}

// AuthURL generates the OAuth2 authorization URL with the given state and nonce parameters.
func (p *OIDCProvider) AuthURL(state, nonce string) string {
	return p.oauth2.AuthCodeURL(state, gooidc.Nonce(nonce))
}

// Exchange trades the authorization code for an ID token and returns a normalized Identity.
// The nonce must match the value embedded in the ID token to prevent replay attacks.
func (p *OIDCProvider) Exchange(ctx context.Context, code, nonce string) (*Identity, error) {
	token, err := p.oauth2.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("exchange code: %w", errIDTokenMissing)
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}

	if idToken.Nonce != nonce {
		return nil, fmt.Errorf("verify id token: %w", errNonceMismatch)
	}

	var claims struct {
		Email   string   `json:"email"`
		Name    string   `json:"name"`
		Picture string   `json:"picture"`
		Groups  []string `json:"groups"`
	}

	if err = idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}

	groups := claims.Groups
	if groups == nil {
		groups = []string{}
	}

	return &Identity{
		Email:   claims.Email,
		Name:    claims.Name,
		Picture: claims.Picture,
		Groups:  groups,
	}, nil
}
