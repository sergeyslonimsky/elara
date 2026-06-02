package integration_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1/authv1connect"
	profilev1 "github.com/sergeyslonimsky/elara/internal/proto/elara/profile/v1"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/profile/v1/profilev1connect"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
	itest "github.com/sergeyslonimsky/elara/test/integration"
)

const (
	adminEmail    = "carol@example.com"
	adminPassword = "unused-password"
)

// basicLoginAndCaptureCookie performs a BasicLogin against the test server and
// returns the elara_session cookie value from the Set-Cookie response header.
func basicLoginAndCaptureCookie(t *testing.T, s *itest.Suite) string {
	t.Helper()

	authClient := authv1connect.NewAuthServiceClient(s.Server.Client(), s.Server.URL)

	resp, err := authClient.BasicLogin(t.Context(), connect.NewRequest(&authv1.BasicLoginRequest{
		Email:    adminEmail,
		Password: adminPassword,
	}))
	require.NoError(t, err)

	cookie := extractSessionCookie(t, resp.Header())
	require.NotEmpty(t, cookie, "expected elara_session cookie after login")

	return cookie
}

// extractSessionCookie returns the value of the elara_session cookie from
// Set-Cookie headers, or "" if not present.
func extractSessionCookie(t *testing.T, h http.Header) string {
	t.Helper()

	for _, c := range h.Values("Set-Cookie") {
		if strings.HasPrefix(c, itest.SessionCookieName+"=") {
			parts := strings.SplitN(c, ";", 2)
			kv := strings.SplitN(parts[0], "=", 2)
			if len(kv) == 2 {
				return kv[1]
			}
		}
	}

	return ""
}

// callMeWithCookie calls ProfileService.Me with the supplied cookie value.
func callMeWithCookie(t *testing.T, s *itest.Suite, cookie string) error {
	t.Helper()

	client := profilev1connect.NewProfileServiceClient(s.Server.Client(), s.Server.URL)
	req := connect.NewRequest(&profilev1.MeRequest{})
	if cookie != "" {
		req.Header().Set("Cookie", itest.SessionCookieName+"="+cookie)
	}

	_, err := client.Me(t.Context(), req)

	return err
}

func TestSession_LoginCookieProtectedGet(t *testing.T) {
	t.Parallel()

	s := itest.New(t)

	cookie := basicLoginAndCaptureCookie(t, s)

	require.NoError(t, callMeWithCookie(t, s, cookie))
}

func TestSession_LoginLogoutThenProtectedGetUnauthorized(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cookie := basicLoginAndCaptureCookie(t, s)

	profClient := profilev1connect.NewProfileServiceClient(s.Server.Client(), s.Server.URL)
	logoutReq := connect.NewRequest(&profilev1.LogoutRequest{})
	logoutReq.Header().Set("Cookie", itest.SessionCookieName+"="+cookie)

	logoutResp, err := profClient.Logout(t.Context(), logoutReq)
	require.NoError(t, err)

	// Logout response must clear the cookie (Max-Age=0 from MaxAge=-1).
	clearCookieFound := false
	for _, c := range logoutResp.Header().Values("Set-Cookie") {
		if strings.Contains(c, itest.SessionCookieName) && strings.Contains(c, "Max-Age=0") {
			clearCookieFound = true

			break
		}
	}
	assert.True(t, clearCookieFound, "logout must return a cleared session cookie")

	err = callMeWithCookie(t, s, cookie)
	require.Error(t, err)

	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeUnauthenticated, connErr.Code())
}

func TestSession_AdminRevokeThenProtectedGetUnauthorized(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cookie := basicLoginAndCaptureCookie(t, s)

	// Bypass the (not-yet-implemented) admin RPC by calling SessionService.Revoke
	// directly with the cookie ID — the cookie IS the session ID for opaque sessions.
	require.NoError(t, s.Managers.Sessions.Revoke(
		t.Context(),
		cookie,
		"admin",
		"test",
		domain.SessionEventRevokedByAdmin,
	))

	err := callMeWithCookie(t, s, cookie)
	require.Error(t, err)

	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeUnauthenticated, connErr.Code())
}

func TestSession_ExpiredSessionRejected(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cookie := basicLoginAndCaptureCookie(t, s)

	// Fast-forward expiry by reading the session, mutating ExpiresAt to the
	// past, and writing it back via the same bbolt repo SessionService uses.
	// This is cleaner than time-mocking and avoids real sleep.
	repo := s.Adapters.SessionRepo

	sess, err := repo.Get(t.Context(), cookie)
	require.NoError(t, err)

	sess.ExpiresAt = time.Now().Add(-time.Hour)
	require.NoError(t, repo.Update(t.Context(), sess))

	err = callMeWithCookie(t, s, cookie)
	require.Error(t, err)

	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodeUnauthenticated, connErr.Code())
}

func TestSession_CLIBearerToken(t *testing.T) {
	t.Parallel()

	s := itest.New(t)

	sess, err := s.Managers.Sessions.Create(t.Context(), sessions.CreateParams{
		UserID:     s.PersonaIDs["admin"],
		ClientType: string(domain.ClientTypeCLI),
	})
	require.NoError(t, err)

	client := profilev1connect.NewProfileServiceClient(s.Server.Client(), s.Server.URL)
	req := connect.NewRequest(&profilev1.MeRequest{})
	req.Header().Set("Authorization", "Bearer "+sess.ID)

	_, err = client.Me(t.Context(), req)
	require.NoError(t, err)
}

func TestSession_ConcurrentRequestsNoRace(t *testing.T) {
	t.Parallel()

	s := itest.New(t)
	cookie := basicLoginAndCaptureCookie(t, s)

	const n = 10

	var wg sync.WaitGroup
	errs := make([]error, n)

	for i := range n {
		wg.Go(func() {
			errs[i] = callMeWithCookie(t, s, cookie)
		})
	}

	wg.Wait()

	for i, err := range errs {
		assert.NoErrorf(t, err, "request %d failed", i)
	}
}
