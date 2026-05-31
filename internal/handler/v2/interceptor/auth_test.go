package interceptor_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/handler/v2/interceptor"
	interceptor_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/interceptor/mocks"
	configv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1"
	"github.com/sergeyslonimsky/elara/internal/proto/elara/config/v1/configv1connect"
)

const testUserEmail = "user@example.com"

func validSession() *domain.Session {
	return &domain.Session{
		ID:         "valid-session-id",
		UserID:     testUserEmail,
		ClientType: domain.ClientTypeWeb,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
	}
}

func activeUser() *domain.User {
	return &domain.User{Email: testUserEmail, Name: "Test User"}
}

// testConfigServer implements a minimal configv1connect.ConfigServiceHandler for testing.
type testConfigServer struct {
	configv1connect.UnimplementedConfigServiceHandler

	called     bool
	wantClaims bool
	t          *testing.T
}

func (s *testConfigServer) GetConfig(
	ctx context.Context,
	_ *connect.Request[configv1.GetConfigRequest],
) (*connect.Response[configv1.GetConfigResponse], error) {
	s.called = true
	if s.wantClaims {
		user, ok := authctx.UserFromContext(ctx)
		require.True(s.t, ok)
		assert.Equal(s.t, testUserEmail, user.Email)
	}

	return connect.NewResponse(&configv1.GetConfigResponse{}), nil
}

// setupTestServer creates an httptest.Server with the AuthInterceptor.
func setupTestServer(
	t *testing.T,
	authI *interceptor.AuthInterceptor,
	wantClaims bool,
) (*httptest.Server, *testConfigServer) {
	t.Helper()

	srv := &testConfigServer{t: t, wantClaims: wantClaims}

	mux := http.NewServeMux()
	path, handler := configv1connect.NewConfigServiceHandler(srv, connect.WithInterceptors(authI))
	mux.Handle(path, handler)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return ts, srv
}

func TestAuthInterceptor_WrapUnary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cookieValue string
		bearer      string
		setupMocks  func(sess *interceptor_mock.MocksessionValidator, users *interceptor_mock.MockuserLookup)
		wantCode    connect.Code
		wantClaims  bool
	}{
		{
			name:        "valid cookie injects user into context",
			cookieValue: "valid-session-id",
			setupMocks: func(sess *interceptor_mock.MocksessionValidator, users *interceptor_mock.MockuserLookup) {
				sess.EXPECT().Validate(gomock.Any(), "valid-session-id").Return(validSession(), nil)
				users.EXPECT().Get(gomock.Any(), testUserEmail).Return(activeUser(), nil)
				sess.EXPECT().Refresh(gomock.Any(), "valid-session-id").Return(nil)
			},
			wantClaims: true,
		},
		{
			name:     "missing cookie returns unauthenticated",
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name:        "invalid/unknown session returns unauthenticated",
			cookieValue: "garbage",
			setupMocks: func(sess *interceptor_mock.MocksessionValidator, _ *interceptor_mock.MockuserLookup) {
				sess.EXPECT().Validate(gomock.Any(), "garbage").Return(nil, domain.ErrSessionNotFound)
			},
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name:        "expired session returns unauthenticated",
			cookieValue: "valid-session-id",
			setupMocks: func(sess *interceptor_mock.MocksessionValidator, _ *interceptor_mock.MockuserLookup) {
				sess.EXPECT().Validate(gomock.Any(), "valid-session-id").Return(nil, domain.ErrSessionExpired)
			},
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name:        "revoked session returns unauthenticated",
			cookieValue: "valid-session-id",
			setupMocks: func(sess *interceptor_mock.MocksessionValidator, _ *interceptor_mock.MockuserLookup) {
				sess.EXPECT().Validate(gomock.Any(), "valid-session-id").Return(nil, domain.ErrSessionRevoked)
			},
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name:        "user lookup fails returns unauthenticated",
			cookieValue: "valid-session-id",
			setupMocks: func(sess *interceptor_mock.MocksessionValidator, users *interceptor_mock.MockuserLookup) {
				sess.EXPECT().Validate(gomock.Any(), "valid-session-id").Return(validSession(), nil)
				users.EXPECT().Get(gomock.Any(), testUserEmail).Return(nil, errors.New("user not found"))
			},
			wantCode: connect.CodeUnauthenticated,
		},
		{
			name:   "bearer token wins over cookie",
			bearer: "bearer-session-id",
			// cookie also set to ensure it is ignored
			cookieValue: "valid-session-id",
			setupMocks: func(sess *interceptor_mock.MocksessionValidator, users *interceptor_mock.MockuserLookup) {
				sess.EXPECT().Validate(gomock.Any(), "bearer-session-id").Return(validSession(), nil)
				users.EXPECT().Get(gomock.Any(), testUserEmail).Return(activeUser(), nil)
				sess.EXPECT().Refresh(gomock.Any(), "bearer-session-id").Return(nil)
			},
			wantClaims: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sess := interceptor_mock.NewMocksessionValidator(ctrl)
			users := interceptor_mock.NewMockuserLookup(ctrl)
			if tc.setupMocks != nil {
				tc.setupMocks(sess, users)
			}

			authI := interceptor.NewAuthInterceptor(sess, users)
			ts, srv := setupTestServer(t, authI, tc.wantClaims)
			client := configv1connect.NewConfigServiceClient(http.DefaultClient, ts.URL)

			req := connect.NewRequest(&configv1.GetConfigRequest{
				Namespace: "ns",
				Path:      "/test",
			})
			if tc.cookieValue != "" {
				req.Header().Set("Cookie", "elara_session="+tc.cookieValue)
			}
			if tc.bearer != "" {
				req.Header().Set("Authorization", "Bearer "+tc.bearer)
			}

			_, err := client.GetConfig(t.Context(), req)

			if tc.wantCode != 0 {
				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				assert.Equal(t, tc.wantCode, connectErr.Code())
				assert.False(t, srv.called)

				return
			}

			require.NoError(t, err)
			assert.True(t, srv.called)
		})
	}
}

func TestAuthInterceptor_WrapStreamingClient(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	sess := interceptor_mock.NewMocksessionValidator(ctrl)
	users := interceptor_mock.NewMockuserLookup(ctrl)
	i := interceptor.NewAuthInterceptor(sess, users)

	called := false
	next := connect.StreamingClientFunc(
		func(_ context.Context, _ connect.Spec) connect.StreamingClientConn {
			called = true

			return nil
		},
	)

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

	protectedProc := "/some.Service/ProtectedMethod"

	tests := []struct {
		name        string
		procedure   string
		cookieValue string
		setupMocks  func(sess *interceptor_mock.MocksessionValidator, users *interceptor_mock.MockuserLookup)
		wantCode    connect.Code
		wantClaims  bool
	}{
		{
			name:        "valid cookie injects user into context",
			procedure:   protectedProc,
			cookieValue: "valid-session-id",
			setupMocks: func(sess *interceptor_mock.MocksessionValidator, users *interceptor_mock.MockuserLookup) {
				sess.EXPECT().Validate(gomock.Any(), "valid-session-id").Return(validSession(), nil)
				users.EXPECT().Get(gomock.Any(), testUserEmail).Return(activeUser(), nil)
				sess.EXPECT().Refresh(gomock.Any(), "valid-session-id").Return(nil)
			},
			wantClaims: true,
		},
		{
			name:      "missing cookie returns unauthenticated",
			procedure: protectedProc,
			wantCode:  connect.CodeUnauthenticated,
		},
		{
			name:        "invalid session returns unauthenticated",
			procedure:   protectedProc,
			cookieValue: "garbage",
			setupMocks: func(sess *interceptor_mock.MocksessionValidator, _ *interceptor_mock.MockuserLookup) {
				sess.EXPECT().Validate(gomock.Any(), "garbage").Return(nil, domain.ErrSessionNotFound)
			},
			wantCode: connect.CodeUnauthenticated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sess := interceptor_mock.NewMocksessionValidator(ctrl)
			users := interceptor_mock.NewMockuserLookup(ctrl)
			if tc.setupMocks != nil {
				tc.setupMocks(sess, users)
			}

			i := interceptor.NewAuthInterceptor(sess, users)

			header := make(http.Header)
			if tc.cookieValue != "" {
				header.Set("Cookie", "elara_session="+tc.cookieValue)
			}

			conn := &stubStreamingHandlerConn{
				procedure: tc.procedure,
				header:    header,
				ctx:       t.Context(),
			}

			called := false
			handler := func(ctx context.Context, _ connect.StreamingHandlerConn) error {
				called = true
				if tc.wantClaims {
					user, ok := authctx.UserFromContext(ctx)
					require.True(t, ok)
					assert.Equal(t, testUserEmail, user.Email)
				}

				return nil
			}

			err := i.WrapStreamingHandler(handler)(t.Context(), conn)

			if tc.wantCode != 0 {
				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				assert.Equal(t, tc.wantCode, connectErr.Code())
				assert.False(t, called)

				return
			}

			require.NoError(t, err)
			assert.True(t, called)
		})
	}
}
