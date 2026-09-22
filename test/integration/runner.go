package integration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// SessionCookieName is the cookie key used by the auth middleware.
const SessionCookieName = "elara_session"

// Option mutates an outgoing request before send.
type Option func(*http.Request)

// SessionCookie returns the raw session token for the given persona, or ""
// for unauthenticated — a service-token-style credential minted for test
// personas, not a JWT.
func SessionCookie(s *Suite, persona string) string { return s.Tokens[persona] }

// WithPersona attaches the session cookie for the given persona to the outgoing
// request. For "unauthenticated" (or any persona without a token) it is a no-op,
// so the request reaches the server with no credentials.
func WithPersona(s *Suite, persona string) Option {
	return func(r *http.Request) {
		token := SessionCookie(s, persona)
		if token == "" {
			return
		}
		r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	}
}

// WithToken attaches an explicit session token as the elara_session cookie.
// Use this for ad-hoc personas created via AddPersona where there's no
// stable key under s.Tokens.
func WithToken(token string) Option {
	return func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	}
}

// DoRequest POSTs body to endpoint on the test server and returns the raw response.
// Caller is responsible for closing resp.Body.
func DoRequest(t *testing.T, s *Suite, endpoint string, body []byte, opts ...Option) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(
		t.Context(), http.MethodPost,
		s.Server.URL+endpoint,
		bytes.NewReader(body),
	)
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")

	for _, opt := range opts {
		opt(req)
	}

	resp, err := s.Server.Client().Do(req)
	require.NoError(t, err)

	return resp
}

// InjectFileAsBase64 reads filePath, base64-encodes the bytes, and assigns the result
// to the named top-level field of a JSON object body. Returns the re-marshaled body.
//
// Use this for proto `bytes` fields whose binary payload is stored as a sibling fixture
// file (so the payload stays diffable instead of an opaque base64 blob in the request).
func InjectFileAsBase64(t *testing.T, body []byte, field, filePath string) []byte {
	t.Helper()

	raw := ReadFile(t, filePath)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(body, &obj), "parsing JSON body for injection")

	obj[field] = base64.StdEncoding.EncodeToString(raw)

	out, err := json.Marshal(obj)
	require.NoError(t, err)

	return out
}
