package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth"
)

// oidcServer is a minimal OIDC provider for testing.
type oidcServer struct {
	server   *httptest.Server
	key      *rsa.PrivateKey
	clientID string
}

func newOIDCServer(t *testing.T, clientID string) *oidcServer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	s := &oidcServer{key: key, clientID: clientID}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	s.server = srv
	t.Cleanup(srv.Close)

	mux.HandleFunc("/.well-known/openid-configuration", s.discovery)
	mux.HandleFunc("/token", s.token)
	mux.HandleFunc("/jwks", s.jwks)

	return s
}

func (s *oidcServer) issuer() string { return s.server.URL }

func (s *oidcServer) discovery(w http.ResponseWriter, _ *http.Request) {
	doc := map[string]any{
		"issuer":                                s.issuer(),
		"authorization_endpoint":                s.issuer() + "/auth",
		"token_endpoint":                        s.issuer() + "/token",
		"jwks_uri":                              s.issuer() + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(doc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *oidcServer) jwks(w http.ResponseWriter, _ *http.Request) {
	jwk := jose.JSONWebKey{Key: &s.key.PublicKey, KeyID: "test-key", Algorithm: string(jose.RS256)}
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(set); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *oidcServer) token(w http.ResponseWriter, r *http.Request) {
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: s.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"),
	)
	if err != nil {
		http.Error(w, "signer error", http.StatusInternalServerError)

		return
	}

	now := time.Now()
	idTokenClaims := struct {
		josejwt.Claims
		Email   string   `json:"email"`
		Name    string   `json:"name"`
		Picture string   `json:"picture"`
		Groups  []string `json:"groups"`
		Nonce   string   `json:"nonce"`
	}{
		Claims: josejwt.Claims{
			Issuer:   s.issuer(),
			Subject:  "user-123",
			Audience: josejwt.Audience{s.clientID},
			IssuedAt: josejwt.NewNumericDate(now),
			Expiry:   josejwt.NewNumericDate(now.Add(time.Hour)),
		},
		Email:   "alice@example.com",
		Name:    "Alice",
		Picture: "https://example.com/alice.png",
		Groups:  []string{"admin"},
		Nonce:   r.FormValue("nonce"),
	}

	rawIDToken, err := josejwt.Signed(sig).Claims(idTokenClaims).Serialize()
	if err != nil {
		http.Error(w, "sign error", http.StatusInternalServerError)

		return
	}

	resp := map[string]any{
		"access_token": "fake-access-token",
		"token_type":   "Bearer",
		"id_token":     rawIDToken,
		"expires_in":   3600,
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func TestNewOIDCProvider_ValidIssuer(t *testing.T) {
	t.Parallel()

	srv := newOIDCServer(t, "test-client")

	cfg := auth.OIDCConfig{
		IssuerURL:    srv.issuer(),
		ClientID:     "test-client",
		ClientSecret: "secret",
		RedirectURL:  "http://localhost/callback",
	}

	p, err := auth.NewOIDCProvider(t.Context(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "oidc", p.Name())
}

func TestNewOIDCProvider_InvalidIssuer(t *testing.T) {
	t.Parallel()

	cfg := auth.OIDCConfig{
		IssuerURL:    "http://127.0.0.1:1",
		ClientID:     "test-client",
		ClientSecret: "secret",
		RedirectURL:  "http://localhost/callback",
	}

	_, err := auth.NewOIDCProvider(t.Context(), cfg)
	require.Error(t, err)
}

func TestOIDCProvider_AuthURL(t *testing.T) {
	t.Parallel()

	srv := newOIDCServer(t, "test-client")

	cfg := auth.OIDCConfig{
		IssuerURL:    srv.issuer(),
		ClientID:     "test-client",
		ClientSecret: "secret",
		RedirectURL:  "http://localhost/callback",
	}

	p, err := auth.NewOIDCProvider(t.Context(), cfg)
	require.NoError(t, err)

	tests := []struct {
		name         string
		state        string
		nonce        string
		wantURLParts []string
	}{
		{
			name:         "basic",
			state:        "state-abc",
			nonce:        "nonce-xyz",
			wantURLParts: []string{"state=state-abc", "nonce=nonce-xyz", "client_id=test-client"},
		},
		{
			name:         "empty state",
			state:        "",
			nonce:        "nonce-xyz",
			wantURLParts: []string{"nonce=nonce-xyz", "client_id=test-client"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			url := p.AuthURL(tc.state, tc.nonce)
			for _, part := range tc.wantURLParts {
				assert.Contains(t, url, part)
			}
		})
	}
}

func TestOIDCProvider_Exchange(t *testing.T) {
	t.Parallel()

	srv := newOIDCServer(t, "test-client")

	cfg := auth.OIDCConfig{
		IssuerURL:    srv.issuer(),
		ClientID:     "test-client",
		ClientSecret: "secret",
		RedirectURL:  "http://localhost/callback",
	}

	p, err := auth.NewOIDCProvider(t.Context(), cfg)
	require.NoError(t, err)

	// The fake server embeds the nonce from the token request form value.
	// In a real flow the nonce is embedded at auth URL time; here we use empty string
	// since the fake /token handler sets nonce from r.FormValue("nonce") which is empty.
	identity, err := p.Exchange(t.Context(), "fake-code", "")
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", identity.Email)
	assert.Equal(t, "Alice", identity.Name)
	assert.Equal(t, "https://example.com/alice.png", identity.Picture)
	assert.Equal(t, []string{"admin"}, identity.Groups)
}

func TestOIDCProvider_DefaultScopes(t *testing.T) {
	t.Parallel()

	srv := newOIDCServer(t, "test-client")

	// No scopes provided — should default to openid, email, profile.
	cfg := auth.OIDCConfig{
		IssuerURL:    srv.issuer(),
		ClientID:     "test-client",
		ClientSecret: "secret",
		RedirectURL:  "http://localhost/callback",
	}

	p, err := auth.NewOIDCProvider(t.Context(), cfg)
	require.NoError(t, err)

	url := p.AuthURL("s", "n")
	assert.Contains(t, url, "scope=")
	assert.Contains(t, url, "openid")
}
