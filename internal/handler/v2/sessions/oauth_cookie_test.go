package sessions_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/handler/v2/sessions"
)

func TestSetOAuthStateCookie(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		value  string
		secure bool
	}{
		{name: "secure", value: "state-123", secure: true},
		{name: "insecure for local dev", value: "state-456", secure: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			header := http.Header{}
			sessions.SetOAuthStateCookie(header, tt.value, tt.secure)

			cookies := (&http.Response{Header: header}).Cookies()
			require.Len(t, cookies, 1)

			c := cookies[0]
			assert.Equal(t, sessions.OAuthStateCookieName, c.Name)
			assert.Equal(t, tt.value, c.Value)
			assert.True(t, c.HttpOnly)
			assert.Equal(t, tt.secure, c.Secure)
			assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
			assert.Equal(t, "/", c.Path)
		})
	}
}

func TestSetOAuthNonceCookie(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		value  string
		secure bool
	}{
		{name: "secure", value: "nonce-123", secure: true},
		{name: "insecure for local dev", value: "nonce-456", secure: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			header := http.Header{}
			sessions.SetOAuthNonceCookie(header, tt.value, tt.secure)

			cookies := (&http.Response{Header: header}).Cookies()
			require.Len(t, cookies, 1)

			c := cookies[0]
			assert.Equal(t, sessions.OAuthNonceCookieName, c.Name)
			assert.Equal(t, tt.value, c.Value)
			assert.True(t, c.HttpOnly)
			assert.Equal(t, tt.secure, c.Secure)
			assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
			assert.Equal(t, "/", c.Path)
		})
	}
}
