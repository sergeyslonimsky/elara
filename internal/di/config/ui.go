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
	Enabled            bool
	Type               AuthType
	AdminEmail         string
	SuperAdminUsername string
	SuperAdminPassword string
	BasicAuth          BasicAuthConfig
	OIDC               OIDCConfig
	Session            SessionConfig
}

type BasicAuthConfig struct {
	AdminInitialPassword string
}

// OIDCConfig holds OpenID Connect provider settings.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
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
	ErrBasicAuthAdminEmailRequired           = errors.New("basic-auth requires ui.auth.adminEmail to be set")
	ErrBasicAuthAdminInitialPasswordRequired = errors.New(
		"basic-auth requires ui.auth.basicAuth.adminInitialPassword to be set",
	)
	ErrSuperAdminUsernameRequired = errors.New("ui.auth.superadmin.username (or SUPERADMIN_USERNAME) is required")
	ErrSuperAdminPasswordRequired = errors.New("ui.auth.superadmin.password (or SUPERADMIN_PASSWORD) is required")
)

// Validate returns an error if the configuration is invalid.
func (c UIAuthConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.SuperAdminUsername == "" {
		return ErrSuperAdminUsernameRequired
	}
	if c.SuperAdminPassword == "" {
		return ErrSuperAdminPasswordRequired
	}

	if c.Type == AuthTypeBasicAuth {
		if c.AdminEmail == "" {
			return ErrBasicAuthAdminEmailRequired
		}
		if c.BasicAuth.AdminInitialPassword == "" {
			return ErrBasicAuthAdminInitialPasswordRequired
		}
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
			Enabled:            cfg.GetBool("ui.auth.enabled"),
			Type:               getAuthType(cfg),
			AdminEmail:         cfg.GetString("ui.auth.adminEmail"),
			SuperAdminUsername: getSuperAdminUsername(cfg),
			SuperAdminPassword: getSuperAdminPassword(cfg),
			BasicAuth: BasicAuthConfig{
				AdminInitialPassword: cfg.GetString("ui.auth.basicAuth.adminInitialPassword"),
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

func getSuperAdminUsername(cfg *di.Config) string {
	u := cfg.GetString("ui.auth.superadmin.username")
	if u != "" {
		return u
	}

	return cfg.GetString("SUPERADMIN_USERNAME")
}

func getSuperAdminPassword(cfg *di.Config) string {
	p := cfg.GetString("ui.auth.superadmin.password")
	if p != "" {
		return p
	}

	return cfg.GetString("SUPERADMIN_PASSWORD")
}
