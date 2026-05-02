package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/auth"
)

func TestIdentity_Fields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		email   string
		uname   string
		picture string
		groups  []string
	}{
		{
			name:    "full identity",
			email:   "alice@example.com",
			uname:   "Alice",
			picture: "https://example.com/alice.png",
			groups:  []string{"admin", "dev"},
		},
		{
			name:   "no groups",
			email:  "bob@example.com",
			uname:  "Bob",
			groups: []string{},
		},
		{
			name:   "empty identity",
			groups: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			id := auth.Identity{
				Email:   tc.email,
				Name:    tc.uname,
				Picture: tc.picture,
				Groups:  tc.groups,
			}

			assert.Equal(t, tc.email, id.Email)
			assert.Equal(t, tc.uname, id.Name)
			assert.Equal(t, tc.picture, id.Picture)
			assert.Equal(t, tc.groups, id.Groups)
		})
	}
}

func TestIdentityProvider_Interface(t *testing.T) {
	t.Parallel()

	// Verify that *OIDCProvider satisfies IdentityProvider at compile time.
	var _ auth.IdentityProvider = (*auth.OIDCProvider)(nil)
}
