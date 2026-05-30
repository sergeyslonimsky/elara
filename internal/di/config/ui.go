package config

import (
	"errors"
	"time"

	"github.com/sergeyslonimsky/core/di"
	"github.com/sergeyslonimsky/core/http2"
)

type AuthType string

const (
	AuthTypeOIDC      AuthType = "oidc"
	AuthTypeBasicAuth AuthType = "basic-auth"
	AuthTypeNone      AuthType = "none"
)

type UI struct {
	Server http2.Config
	Auth   UIAuthConfig
}

// UIAuthConfig controls authentication and session management.
type UIAuthConfig struct {
	Enabled   bool
	Type      AuthType
	BasicAuth BasicAuthConfig
	OIDC      OIDCConfig
	Session   SessionConfig
}

// BasicAuthConfig holds the initial admin credentials used when Type == basic-auth.
// On bootstrap, a local user with these credentials is created and added to the
// superadmin group.
type BasicAuthConfig struct {
	Username string
	Password string
}

// OIDCConfig holds OpenID Connect provider settings.
// AdminEmail bootstraps the first superadmin identity for OIDC: the first time
// a user with this email completes the OIDC callback, they are added to the
// superadmin group.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	AdminEmail   string
}

// SessionConfig controls JWT session token signing and lifetime.
type SessionConfig struct {
	Secret string
	TTL    time.Duration
	// SecureCookie marks the session cookie with the Secure flag.
	// Set to true only when the service is served over HTTPS.
	// On plain HTTP (e.g. local dev) the browser drops Secure cookies per RFC 6265.
	SecureCookie bool
}

var (
	ErrBasicAuthUsernameRequired = errors.New(
		"basic-auth requires ui.auth.basicAuth.username to be set",
	)
	ErrBasicAuthPasswordRequired = errors.New(
		"basic-auth requires ui.auth.basicAuth.password to be set",
	)
	ErrOIDCAdminEmailRequired = errors.New("oidc requires ui.auth.oidc.adminEmail to be set")
)

// Validate returns an error if the configuration is invalid.
func (c UIAuthConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	switch c.Type {
	case AuthTypeBasicAuth:
		if c.BasicAuth.Username == "" {
			return ErrBasicAuthUsernameRequired
		}
		if c.BasicAuth.Password == "" {
			return ErrBasicAuthPasswordRequired
		}
	case AuthTypeOIDC:
		if c.OIDC.AdminEmail == "" {
			return ErrOIDCAdminEmailRequired
		}
	case AuthTypeNone:
		// no admin identity required — passthrough user is seeded at bootstrap
	}

	return nil
}

func newUIConfig(cfg *di.Config) (UI, error) {
	ui := UI{
		Server: http2.Config{
			Port:        cfg.GetStringOrDefault("ui.server.port", defaultHTTPPort),
			ReadTimeout: cfg.GetDuration("ui.server.readTimeout"),
			// Streaming-friendly default — see defaultFrontendWriteTimeout.
			WriteTimeout: durOrDefault(
				cfg.GetDuration("ui.server.writeTimeout"),
				defaultFrontendWriteTimeout,
			),
		},
		Auth: UIAuthConfig{
			Enabled: cfg.GetBool("ui.auth.enabled"),
			Type:    getAuthType(cfg),
			BasicAuth: BasicAuthConfig{
				Username: cfg.GetString("ui.auth.basicAuth.username"),
				Password: cfg.GetString("ui.auth.basicAuth.password"),
			},
			OIDC: OIDCConfig{
				IssuerURL:    cfg.GetString("ui.auth.oidc.issuerUrl"),
				ClientID:     cfg.GetString("ui.auth.oidc.clientId"),
				ClientSecret: cfg.GetString("ui.auth.oidc.clientSecret"),
				RedirectURL:  cfg.GetString("ui.auth.oidc.redirectUrl"),
				Scopes: stringsOrDefault(
					cfg.GetStringSlice("ui.auth.oidc.scopes"),
					[]string{"openid", "email", "profile"},
				),
				AdminEmail: cfg.GetString("ui.auth.oidc.adminEmail"),
			},
			Session: SessionConfig{
				Secret: cfg.GetString("ui.auth.session.secret"),
				TTL: durOrDefault(
					cfg.GetDuration("ui.auth.session.ttl"),
					defaultSessionTTL,
				),
				SecureCookie: cfg.GetBool("ui.auth.session.secureCookie"),
			},
		},
	}

	if err := ui.Auth.Validate(); err != nil {
		return UI{}, err
	}

	return ui, nil
}

func getAuthType(cfg *di.Config) AuthType {
	if !cfg.GetBool("ui.auth.enabled") {
		return AuthTypeNone
	}

	authType := cfg.GetString("ui.auth.type")

	switch authType {
	case "oidc":
		return AuthTypeOIDC
	case "basic-auth":
		return AuthTypeBasicAuth
	case "none":
		return AuthTypeNone
	default:
		return AuthTypeNone
	}
}
