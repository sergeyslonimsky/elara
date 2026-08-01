package sessions_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/handler/v2/sessions"
)

func TestSetSessionCookie(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sessionID string
		expiresAt time.Time
		secure    bool
	}{
		{
			name:      "secure cookie",
			sessionID: "sess-abc",
			expiresAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			secure:    true,
		},
		{
			name:      "insecure cookie for local dev",
			sessionID: "sess-def",
			expiresAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			secure:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			header := http.Header{}
			sessions.SetSessionCookie(header, tt.sessionID, tt.expiresAt, tt.secure)

			raw := header.Get("Set-Cookie")
			require.NotEmpty(t, raw)

			cookies := (&http.Response{Header: header}).Cookies()
			require.Len(t, cookies, 1)

			c := cookies[0]
			assert.Equal(t, sessions.CookieName, c.Name)
			assert.Equal(t, tt.sessionID, c.Value)
			assert.True(t, c.HttpOnly)
			assert.Equal(t, tt.secure, c.Secure)
			assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
			assert.Equal(t, "/", c.Path)
			assert.True(t, tt.expiresAt.Equal(c.Expires))
		})
	}
}

func TestClearSessionCookie(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		secure bool
	}{
		{name: "secure", secure: true},
		{name: "insecure", secure: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			header := http.Header{}
			sessions.ClearSessionCookie(header, tt.secure)

			cookies := (&http.Response{Header: header}).Cookies()
			require.Len(t, cookies, 1)

			c := cookies[0]
			assert.Equal(t, sessions.CookieName, c.Name)
			assert.Empty(t, c.Value)
			assert.True(t, c.HttpOnly)
			assert.Equal(t, tt.secure, c.Secure)
			assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
			assert.Equal(t, "/", c.Path)
			assert.Equal(t, -1, c.MaxAge)
		})
	}
}
