package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestNewIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		provider    domain.IdentityProvider
		rawSubject  string
		want        domain.Identity
		wantErr     bool
		errIs       error
		errContains string
	}{
		{
			name:       "basic lowercases email",
			provider:   domain.ProviderBasic,
			rawSubject: "Admin@Example.Com",
			want: domain.Identity{
				Provider: domain.ProviderBasic,
				Subject:  "admin@example.com",
			},
		},
		{
			name:       "basic trims whitespace",
			provider:   domain.ProviderBasic,
			rawSubject: "  alice@example.com  ",
			want: domain.Identity{
				Provider: domain.ProviderBasic,
				Subject:  "alice@example.com",
			},
		},
		{
			name:       "basic NFKC-folds fullwidth latin",
			provider:   domain.ProviderBasic,
			rawSubject: "ＡＤＭＩＮ@example.com",
			want: domain.Identity{
				Provider: domain.ProviderBasic,
				Subject:  "admin@example.com",
			},
		},
		{
			name:        "basic rejects username without @",
			provider:    domain.ProviderBasic,
			rawSubject:  "admin",
			wantErr:     true,
			errContains: "normalize basic subject",
		},
		{
			name:       "basic rejects missing local part",
			provider:   domain.ProviderBasic,
			rawSubject: "@example.com",
			wantErr:    true,
		},
		{
			name:       "basic rejects missing domain part",
			provider:   domain.ProviderBasic,
			rawSubject: "admin@",
			wantErr:    true,
		},
		{
			name:       "basic rejects multiple @",
			provider:   domain.ProviderBasic,
			rawSubject: "a@b@c",
			wantErr:    true,
		},
		{
			name:       "oidc preserves opaque sub verbatim",
			provider:   domain.ProviderOIDC,
			rawSubject: "Mixed-Case_Sub.123|RAW",
			want: domain.Identity{
				Provider: domain.ProviderOIDC,
				Subject:  "Mixed-Case_Sub.123|RAW",
			},
		},
		{
			name:       "oidc multi-issuer preserves opaque sub",
			provider:   domain.OIDCProvider("auth0-tenant-a"),
			rawSubject: "auth0|abc-XYZ",
			want: domain.Identity{
				Provider: "oidc:auth0-tenant-a",
				Subject:  "auth0|abc-XYZ",
			},
		},
		{
			name:       "empty subject rejected for basic",
			provider:   domain.ProviderBasic,
			rawSubject: "",
			wantErr:    true,
		},
		{
			name:       "empty subject rejected for oidc",
			provider:   domain.ProviderOIDC,
			rawSubject: "",
			wantErr:    true,
		},
		{
			name:       "unknown provider rejected",
			provider:   domain.IdentityProvider("ldap"),
			rawSubject: "uid=alice",
			wantErr:    true,
			errIs:      domain.ErrInvalidIdentityProvider,
		},
		{
			name:       "provider with oidc-like prefix but no colon is rejected",
			provider:   domain.IdentityProvider("oidcx"),
			rawSubject: "sub-1",
			wantErr:    true,
			errIs:      domain.ErrInvalidIdentityProvider,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.NewIdentity(tt.provider, tt.rawSubject)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errIs != nil {
					require.ErrorIs(t, err, tt.errIs)
				}
				if tt.errContains != "" {
					assert.Contains(
						t,
						err.Error(), tt.errContains,
						"error %q must contain %q", err.Error(), tt.errContains,
					)
				}
				assert.Equal(t, domain.Identity{}, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNewIdentity_Idempotent guards that running NewIdentity twice on an
// already-normalized subject is a no-op — the lookup path uses NormalizeEmail
// independently, and divergence between the two would re-introduce the
// case-sensitivity bug NewIdentity is meant to close.
func TestNewIdentity_Idempotent(t *testing.T) {
	t.Parallel()

	first, err := domain.NewIdentity(domain.ProviderBasic, "Operator@Corp.IO")
	require.NoError(t, err)

	second, err := domain.NewIdentity(domain.ProviderBasic, first.Subject)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

// TestNewIdentity_InvalidProviderIsProgrammingError guards that
// ErrInvalidIdentityProvider is wrapped via fmt.Errorf with %w so callers
// can errors.Is-check it.
func TestNewIdentity_InvalidProviderIsProgrammingError(t *testing.T) {
	t.Parallel()

	_, err := domain.NewIdentity(domain.IdentityProvider("saml"), "uid")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidIdentityProvider)
}
