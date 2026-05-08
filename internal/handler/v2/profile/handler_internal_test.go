package profile

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	internalauth "github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	profilev1 "github.com/sergeyslonimsky/elara/internal/proto/elara/profile/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

// stubEnforcer is a minimal meEnforcer stub for handler tests.
type stubEnforcer struct {
	allowAll bool
}

func (s *stubEnforcer) Enforce(_, _, _, _ string) (bool, error) {
	return s.allowAll, nil
}

// stubNamespaceLister returns a fixed list of namespaces.
type stubNamespaceLister struct {
	namespaces []*domain.Namespace
}

func (s *stubNamespaceLister) List(_ context.Context) ([]*domain.Namespace, error) {
	return s.namespaces, nil
}

func TestProfileHandler_Me(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		email    string
		authCtx  bool
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name:    "returns user identity",
			email:   "alice@example.com",
			authCtx: true,
		},
		{
			name:     "no auth context returns unauthenticated",
			authCtx:  false,
			wantErr:  true,
			wantCode: connect.CodeUnauthenticated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			enforcer := &stubEnforcer{allowAll: false}
			nsList := &stubNamespaceLister{namespaces: nil}
			meUC := authuc.NewMeUseCase(enforcer, nsList)
			h := New(meUC, nil, config.AuthTypeOIDC, false)

			ctx := context.Background()
			if tc.authCtx {
				ctx = internalauth.WithClaims(ctx, &internalauth.Claims{Email: tc.email, Name: "Alice"})
			}

			resp, err := h.Me(ctx, connect.NewRequest(&profilev1.MeRequest{}))

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.email, resp.Msg.GetEmail())
		})
	}
}

func TestProfileHandler_ChangePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		authType   config.AuthType
		setupMocks func(reader *auth_mock.MockpasswordReader, writer *auth_mock.MockpasswordWriter, session *auth_mock.MocksessionCreator)
		wantErr    bool
	}{
		{
			name:     "success",
			authType: config.AuthTypeBasicAuth,
			setupMocks: func(reader *auth_mock.MockpasswordReader, writer *auth_mock.MockpasswordWriter, session *auth_mock.MocksessionCreator) {
				reader.EXPECT().
					Get(gomock.Any(), "user@example.com").
					Return(&domain.User{Email: "user@example.com"}, nil)
				writer.EXPECT().SetPassword(gomock.Any(), "user@example.com", gomock.Any(), false).Return(nil)
				session.EXPECT().Create(gomock.Any()).Return("mock-token", nil)
			},
		},
		{
			name:     "returns error when auth type is not basic",
			authType: config.AuthTypeOIDC,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			reader := auth_mock.NewMockpasswordReader(ctrl)
			writer := auth_mock.NewMockpasswordWriter(ctrl)
			session := auth_mock.NewMocksessionCreator(ctrl)

			if tt.setupMocks != nil {
				tt.setupMocks(reader, writer, session)
			}

			changeUC := authuc.NewChangePasswordUseCase(reader, writer, session)
			h := New(nil, changeUC, tt.authType, false)

			ctx := internalauth.WithClaims(t.Context(), &internalauth.Claims{
				Email:                  "user@example.com",
				PasswordChangeRequired: true,
			})

			req := connect.NewRequest(&profilev1.ChangePasswordRequest{
				NewPassword: "new-password",
			})

			_, err := h.ChangePassword(ctx, req)

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestProfileHandler_Logout(t *testing.T) {
	t.Parallel()

	h := New(nil, nil, config.AuthTypeOIDC, false)

	resp, err := h.Logout(context.Background(), connect.NewRequest(&profilev1.LogoutRequest{}))
	require.NoError(t, err)

	cookies := resp.Header().Values(cookieHeader)
	require.Len(t, cookies, 1, "expected session-clearing cookie")
	assert.Contains(t, cookies[0], sessionCookieName)
	// MaxAge=-1 is serialized as "Max-Age=0" by net/http.
	assert.Contains(t, cookies[0], "Max-Age=0")
}
