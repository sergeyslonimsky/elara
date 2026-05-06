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
			name: "basic-auth with all fields",
			auth: config.UIAuthConfig{
				Enabled:    true,
				Type:       config.AuthTypeBasicAuth,
				AdminEmail: "admin@example.com",
				BasicAuth: config.BasicAuthConfig{
					AdminInitialPassword: "password",
				},
			},
			wantErr: false,
		},
		{
			name: "basic-auth missing email",
			auth: config.UIAuthConfig{
				Enabled: true,
				Type:    config.AuthTypeBasicAuth,
				BasicAuth: config.BasicAuthConfig{
					AdminInitialPassword: "password",
				},
			},
			wantErr: true,
			errMsg:  "basic-auth requires ui.auth.adminEmail to be set",
		},
		{
			name: "basic-auth missing password",
			auth: config.UIAuthConfig{
				Enabled:    true,
				Type:       config.AuthTypeBasicAuth,
				AdminEmail: "admin@example.com",
			},
			wantErr: true,
			errMsg:  "basic-auth requires ui.auth.basicAuth.adminInitialPassword to be set",
		},
		{
			name: "oidc missing fields (not validated yet)",
			auth: config.UIAuthConfig{
				Enabled: true,
				Type:    config.AuthTypeOIDC,
			},
			wantErr: false,
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
