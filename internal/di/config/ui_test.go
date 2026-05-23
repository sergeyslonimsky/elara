package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/di/config"
)

func TestUIAuthConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		auth    config.UIAuthConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "disabled auth",
			auth: config.UIAuthConfig{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "none type requires nothing",
			auth: config.UIAuthConfig{
				Enabled: true,
				Type:    config.AuthTypeNone,
			},
			wantErr: false,
		},
		{
			name: "basic-auth with username and password",
			auth: config.UIAuthConfig{
				Enabled: true,
				Type:    config.AuthTypeBasicAuth,
				BasicAuth: config.BasicAuthConfig{
					Username: "admin",
					Password: "password",
				},
			},
			wantErr: false,
		},
		{
			name: "basic-auth missing username",
			auth: config.UIAuthConfig{
				Enabled: true,
				Type:    config.AuthTypeBasicAuth,
				BasicAuth: config.BasicAuthConfig{
					Password: "password",
				},
			},
			wantErr: true,
			errMsg:  "basic-auth requires ui.auth.basicAuth.username to be set",
		},
		{
			name: "basic-auth missing password",
			auth: config.UIAuthConfig{
				Enabled: true,
				Type:    config.AuthTypeBasicAuth,
				BasicAuth: config.BasicAuthConfig{
					Username: "admin",
				},
			},
			wantErr: true,
			errMsg:  "basic-auth requires ui.auth.basicAuth.password to be set",
		},
		{
			name: "oidc with admin email",
			auth: config.UIAuthConfig{
				Enabled: true,
				Type:    config.AuthTypeOIDC,
				OIDC: config.OIDCConfig{
					AdminEmail: "admin@example.com",
				},
			},
			wantErr: false,
		},
		{
			name: "oidc missing admin email",
			auth: config.UIAuthConfig{
				Enabled: true,
				Type:    config.AuthTypeOIDC,
			},
			wantErr: true,
			errMsg:  "oidc requires ui.auth.oidc.adminEmail to be set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.auth.Validate()
			if tt.wantErr {
				require.ErrorContains(t, err, tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
