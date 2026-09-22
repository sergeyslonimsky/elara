package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestResolveAuthType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		enabled bool
		raw     string
		want    domain.AuthType
		wantErr bool
	}{
		{name: "disabled ignores raw value", enabled: false, raw: "garbage", want: domain.AuthTypeNone},
		{name: "disabled with empty raw", enabled: false, raw: "", want: domain.AuthTypeNone},
		{
			name:    "enabled, empty raw (Helm's unconditional UI_AUTH_TYPE)",
			enabled: true,
			raw:     "",
			want:    domain.AuthTypeNone,
		},
		{name: "enabled, explicit none (passthrough mode)", enabled: true, raw: "none", want: domain.AuthTypeNone},
		{name: "enabled, oidc", enabled: true, raw: "oidc", want: domain.AuthTypeOIDC},
		{name: "enabled, basic-auth", enabled: true, raw: "basic-auth", want: domain.AuthTypeBasicAuth},
		{name: "enabled, unrecognized non-empty value fails fast", enabled: true, raw: "oidcc", wantErr: true},
		{name: "enabled, case-sensitive mismatch fails fast", enabled: true, raw: "OIDC", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveAuthType(tt.enabled, tt.raw)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrUIAuthTypeUnrecognized)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
