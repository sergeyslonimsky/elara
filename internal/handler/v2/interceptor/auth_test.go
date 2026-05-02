package interceptor_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/interceptor"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1/configv1connect"
)

const testSessionSecret = "test-secret-that-is-long-enough-32b"

func newTestSessionManager() *auth.SessionManager {
	return auth.NewSessionManager(testSessionSecret, time.Hour)
}

func newValidToken(t *testing.T, sm *auth.SessionManager) string {
	t.Helper()

	token, err := sm.Create(&domain.User{Email: "user@example.com", Name: "Test User"})
	require.NoError(t, err)

	return token
}

func newExpiredToken(t *testing.T) string {
	t.Helper()

	expiredSM := auth.NewSessionManager(testSessionSecret, -time.Hour)
	token, err := expiredSM.Create(&domain.User{Email: "user@example.com", Name: "Test"})
	require.NoError(t, err)

	return token
}

// testConfigServer implements a minimal configv1connect.ConfigServiceHandler for testing.
type testConfigServer struct {
	configv1connect.UnimplementedConfigServiceHandler
	capturedCtx context.Context //nolint:containedctx // test helper; context captured for assertion only
}

func (s *testConfigServer) GetConfig(
	ctx context.Context,
	_ *connect.Request[configv1.GetConfigRequest],
) (*connect.Response[configv1.GetConfigResponse], error) {
	s.capturedCtx = ctx

	return connect.NewResponse(&configv1.GetConfigResponse{}), nil
}

// setupTestServer creates an httptest.Server with the AuthInterceptor.
func setupTestServer(
	t *testing.T,
	sm *auth.SessionManager,
	publicProcs []string,
) (*httptest.Server, *testConfigServer) {
	t.Helper()

	srv := &testConfigServer{}
	authI := interceptor.NewAuthInterceptor(sm, publicProcs)

	mux := http.NewServeMux()
	path, handler := configv1connect.NewConfigServiceHandler(srv, connect.WithInterceptors(authI))
	mux.Handle(path, handler)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return ts, srv
}

func TestAuthInterceptor_WrapUnary(t *testing.T) {
	t.Parallel()

	sm := newTestSessionManager()

	tests := []struct {
		name        string
		cookieValue string
		publicProcs []string
		wantCode    connect.Code
		wantClaims  bool
	}{
		{
			name:        "valid cookie injects claims",
			cookieValue: func() string { return newValidToken(t, sm) }(),
			wantClaims:  true,
		},
		{
			name:     "missing cookie returns unauthenticated",
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name:        "invalid cookie returns unauthenticated",
			cookieValue: "not-a-valid-jwt",
			wantCode:    connect.CodeUnauthenticated,
		},
		{
			name:        "expired cookie returns unauthenticated",
			cookieValue: func() string { return newExpiredToken(t) }(),
			wantCode:    connect.CodeUnauthenticated,
		},
		{
			name:        "public procedure bypasses auth",
			publicProcs: []string{configv1connect.ConfigServiceGetConfigProcedure},
			wantClaims:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts, srv := setupTestServer(t, sm, tc.publicProcs)
			client := configv1connect.NewConfigServiceClient(http.DefaultClient, ts.URL)

			req := connect.NewRequest(&configv1.GetConfigRequest{
				Namespace: "ns",
				Path:      "/test",
			})
			if tc.cookieValue != "" {
				req.Header().Set("Cookie", "elara_session="+tc.cookieValue)
			}

			_, err := client.GetConfig(t.Context(), req)

			if tc.wantCode != 0 {
				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				assert.Equal(t, tc.wantCode, connectErr.Code())

				return
			}

			require.NoError(t, err)

			if tc.wantClaims {
				claims, ok := auth.ClaimsFromContext(srv.capturedCtx)
				require.True(t, ok)
				assert.Equal(t, "user@example.com", claims.Email)
			}
		})
	}
}

func TestAuthInterceptor_WrapStreamingClient(t *testing.T) {
	t.Parallel()

	sm := newTestSessionManager()
	i := interceptor.NewAuthInterceptor(sm, nil)

	called := false
	next := connect.StreamingClientFunc(func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		called = true

		return nil
	})

	i.WrapStreamingClient(next)(t.Context(), connect.Spec{})
	assert.True(t, called)
}

// stubStreamingHandlerConn is a minimal streaming handler conn for testing WrapStreamingHandler.
type stubStreamingHandlerConn struct {
	connect.StreamingHandlerConn
	procedure string
	header    http.Header
	ctx       context.Context //nolint:containedctx // test helper; context stored to implement the interface
}

func (s *stubStreamingHandlerConn) Spec() connect.Spec         { return connect.Spec{Procedure: s.procedure} }
func (s *stubStreamingHandlerConn) RequestHeader() http.Header { return s.header }
func (s *stubStreamingHandlerConn) Context() context.Context   { return s.ctx }

func TestAuthInterceptor_WrapStreamingHandler(t *testing.T) {
	t.Parallel()

	sm := newTestSessionManager()
	publicProc := "/some.Service/PublicMethod"
	protectedProc := "/some.Service/ProtectedMethod"

	tests := []struct {
		name        string
		procedure   string
		cookieValue string
		wantCode    connect.Code
		wantClaims  bool
	}{
		{
			name:       "public procedure bypasses auth",
			procedure:  publicProc,
			wantClaims: false,
		},
		{
			name:        "valid cookie injects claims into context",
			procedure:   protectedProc,
			cookieValue: func() string { return newValidToken(t, sm) }(),
			wantClaims:  true,
		},
		{
			name:      "missing cookie returns unauthenticated",
			procedure: protectedProc,
			wantCode:  connect.CodeUnauthenticated,
		},
		{
			name:        "invalid cookie returns unauthenticated",
			procedure:   protectedProc,
			cookieValue: "garbage",
			wantCode:    connect.CodeUnauthenticated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			i := interceptor.NewAuthInterceptor(sm, []string{publicProc})

			header := make(http.Header)
			if tc.cookieValue != "" {
				header.Set("Cookie", "elara_session="+tc.cookieValue)
			}

			conn := &stubStreamingHandlerConn{
				procedure: tc.procedure,
				header:    header,
				ctx:       t.Context(),
			}

			var capturedCtx context.Context
			handler := func(ctx context.Context, c connect.StreamingHandlerConn) error {
				capturedCtx = ctx //nolint:fatcontext // test helper; context captured for assertion only

				return nil
			}

			err := i.WrapStreamingHandler(handler)(t.Context(), conn)

			if tc.wantCode != 0 {
				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				assert.Equal(t, tc.wantCode, connectErr.Code())

				return
			}

			require.NoError(t, err)

			if tc.wantClaims {
				claims, ok := auth.ClaimsFromContext(capturedCtx)
				require.True(t, ok)
				assert.Equal(t, "user@example.com", claims.Email)
			}
		})
	}
}
