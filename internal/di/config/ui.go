package config

import (
	"time"

	"github.com/sergeyslonimsky/core/di"
	"github.com/sergeyslonimsky/core/http2"
)

type AuthType string

const (
	AuthTypeOIDC      AuthType = "oidc"
	AuthTypeBasicAuth AuthType = "basic-auth"
)

type UI struct {
	Server http2.Config
	Auth   UIAuthConfig
}

// UIAuthConfig controls authentication and session management.
type UIAuthConfig struct {
	Enabled     bool
	Type        AuthType
	AdminEmails []string
	OIDC        OIDCConfig
	Session     SessionConfig
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
}

func newUIConfig(cfg *di.Config) UI {
	return UI{
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
			Enabled:     cfg.GetBool("ui.auth.enabled"),
			Type:        getAuthType(cfg),
			AdminEmails: cfg.GetStringSlice("ui.auth.adminEmails"),
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
			},
		},
	}
}

func getAuthType(cfg *di.Config) AuthType {
	authType := cfg.GetString("ui.auth.type")

	switch authType {
	case "oidc":
		return AuthTypeOIDC
	case "basic-auth":
		return AuthTypeBasicAuth
	default:
		return AuthTypeBasicAuth
	}
}
