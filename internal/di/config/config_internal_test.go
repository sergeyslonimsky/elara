package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestConfig_ValidateSkipPermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "flag off, both auth surfaces on: irrelevant, no error",
			cfg: Config{
				DangerouslySkipPermissions: false,
				UI:                         UI{Auth: UIAuthConfig{Enabled: true}},
				Client:                     Client{Auth: ClientAuth{Enabled: true}},
			},
		},
		{
			name: "flag on, both auth surfaces off: the intended open-dev-instance case",
			cfg: Config{
				DangerouslySkipPermissions: true,
				UI:                         UI{Auth: UIAuthConfig{Enabled: false}},
				Client:                     Client{Auth: ClientAuth{Enabled: false}},
			},
		},
		{
			name: "flag on + ui.auth.enabled: rejected",
			cfg: Config{
				DangerouslySkipPermissions: true,
				UI:                         UI{Auth: UIAuthConfig{Enabled: true}},
				Client:                     Client{Auth: ClientAuth{Enabled: false}},
			},
			wantErr: true,
		},
		{
			name: "flag on + client.auth.enabled: rejected",
			cfg: Config{
				DangerouslySkipPermissions: true,
				UI:                         UI{Auth: UIAuthConfig{Enabled: false}},
				Client:                     Client{Auth: ClientAuth{Enabled: true}},
			},
			wantErr: true,
		},
		{
			name: "flag on + both auth surfaces on: rejected",
			cfg: Config{
				DangerouslySkipPermissions: true,
				UI:                         UI{Auth: UIAuthConfig{Enabled: true}},
				Client:                     Client{Auth: ClientAuth{Enabled: true}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.cfg.validateSkipPermissions()
			if tt.wantErr {
				require.ErrorIs(t, err, ErrDangerousSkipPermissionsWithRealAuth)

				return
			}

			require.NoError(t, err)
		})
	}
}

// TestConfig_ShouldSkipPermissionsForUI is a regression test for a real
// lockout bug found in review: ui.auth.enabled=true + ui.auth.type=none
// ("passthrough" mode, which UIAuthConfig.Validate() documents as
// legitimate) must skip permission enforcement exactly like
// ui.auth.enabled=false does — otherwise PDP enforces against the
// AuthInterceptor's synthetic bypass principal, which has no Casbin policy,
// and every request is denied.
func TestConfig_ShouldSkipPermissionsForUI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{
			name: "auth disabled (resolveAuthType always resolves Type to None here)",
			cfg:  Config{UI: UI{Auth: UIAuthConfig{Enabled: false, Type: domain.AuthTypeNone}}},
			want: true,
		},
		{
			name: "auth enabled, passthrough (type=none) — the bug this regresses against",
			cfg:  Config{UI: UI{Auth: UIAuthConfig{Enabled: true, Type: domain.AuthTypeNone}}},
			want: true,
		},
		{
			name: "auth enabled, basic-auth: enforced",
			cfg:  Config{UI: UI{Auth: UIAuthConfig{Enabled: true, Type: domain.AuthTypeBasicAuth}}},
			want: false,
		},
		{
			name: "auth enabled, oidc: enforced",
			cfg:  Config{UI: UI{Auth: UIAuthConfig{Enabled: true, Type: domain.AuthTypeOIDC}}},
			want: false,
		},
		{
			name: "basic-auth configured but DangerouslySkipPermissions set: bypassed",
			cfg: Config{
				UI:                         UI{Auth: UIAuthConfig{Enabled: true, Type: domain.AuthTypeBasicAuth}},
				DangerouslySkipPermissions: true,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.cfg.ShouldSkipPermissionsForUI())
		})
	}
}
