package integration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// SessionCookieName is the cookie key used by the auth middleware.
const SessionCookieName = "elara_session"

// Option mutates an outgoing request before send.
type Option func(*http.Request)

// WithHeader sets a request header.
func WithHeader(key, value string) Option {
	return func(r *http.Request) { r.Header.Set(key, value) }
}

// WithCookie attaches a single cookie via the Cookie header.
func WithCookie(name, value string) Option {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: name, Value: value}) }
}

// SessionCookie returns the raw session JWT for the given persona, or "" for unauthenticated.
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

// WithToken attaches an explicit session JWT as the elara_session cookie.
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

// TC is one integration sub-test: load reqPath verbatim, send as persona, compare body to respPath.
type TC struct {
	Name    string
	Persona string
	Req     string
	Resp    string
}

// RunCases is sugar for the common pattern. For cases that need pre-processing
// (e.g. binary field injection), drop to the primitives instead.
func RunCases(t *testing.T, endpoint string, cases []TC) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			Run(t, New(t), endpoint, tc.Persona, tc.Req, tc.Resp)
		})
	}
}

// Run executes a single case: load reqPath verbatim, POST as persona, compare resp to respPath.
func Run(t *testing.T, s *Suite, endpoint, persona, reqPath, respPath string) {
	t.Helper()

	body := ReadFile(t, reqPath)

	var opts []Option
	if c := SessionCookie(s, persona); c != "" {
		opts = append(opts, WithCookie(SessionCookieName, c))
	}

	resp := DoRequest(t, s, endpoint, body, opts...)
	defer func() { _ = resp.Body.Close() }()

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	CompareJSONBytes(t, ReadFile(t, respPath), got)
}
