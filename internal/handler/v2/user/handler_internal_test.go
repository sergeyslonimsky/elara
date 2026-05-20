package user

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	user_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/user/mocks"
	userv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/user/v1"
	useruc "github.com/sergeyslonimsky/elara/internal/usecase/user"
)

func TestUserHandler_ListUsers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *Handler
		wantLen  int
		wantErr  bool
	}{
		{
			name: "returns all users",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().List(gomock.Any(), useruc.ListParams{}).
					Return(&useruc.ListResult{
						Users: []*domain.User{{Email: "a@example.com"}, {Email: "b@example.com"}},
						Total: 2,
					}, nil)

				return New(uc, config.AuthTypeOIDC)
			},
			wantLen: 2,
		},
		{
			name: "storage error returns internal",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().List(gomock.Any(), useruc.ListParams{}).Return(nil, errors.New("db error"))

				return New(uc, config.AuthTypeOIDC)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.ListUsers(t.Context(), connect.NewRequest(&userv1.ListUsersRequest{}))

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetUsers(), tt.wantLen)
		})
	}
}

func TestUserHandler_GetUser(t *testing.T) {
	t.Parallel()

	email := "alice@example.com"

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *Handler
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name: "returns user by email",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Get(gomock.Any(), email).
					Return(&domain.User{Email: email, Name: "Alice"}, nil)

				return New(uc, config.AuthTypeOIDC)
			},
		},
		{
			name: "not found returns NotFound code",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Get(gomock.Any(), email).Return(nil, domain.ErrNotFound)

				return New(uc, config.AuthTypeOIDC)
			},
			wantErr:  true,
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			resp, err := h.GetUser(t.Context(), connect.NewRequest(&userv1.GetUserRequest{Email: email}))

			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, email, resp.Msg.GetUser().GetEmail())
		})
	}
}

func TestUserHandler_DeleteUser(t *testing.T) {
	t.Parallel()

	targetEmail := "target@example.com"

	tests := []struct {
		name      string
		authType  config.AuthType
		mockFunc  func(*gomock.Controller) *Handler
		wantErr   bool
		wantErrMs string
		wantCode  connect.Code
	}{
		{
			name:     "authType == OIDC returns ErrFeatureNotAvailable",
			authType: config.AuthTypeOIDC,
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				uc := user_mock.NewMockusecase(ctrl)

				return New(uc, config.AuthTypeOIDC)
			},
			wantErr:   true,
			wantCode:  connect.CodeInvalidArgument,
			wantErrMs: "feature not available",
		},
		{
			name:     "authType == BasicAuth: success",
			authType: config.AuthTypeBasicAuth,
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().Delete(gomock.Any(), targetEmail).Return(nil)

				return New(uc, config.AuthTypeBasicAuth)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			_, err := h.DeleteUser(t.Context(), connect.NewRequest(&userv1.DeleteUserRequest{Email: targetEmail}))

			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))
				if tt.wantErrMs != "" {
					assert.Contains(t, err.Error(), tt.wantErrMs)
				}

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestUserHandler_CreateUser(t *testing.T) {
	t.Parallel()

	email := "user@example.com"

	tests := []struct {
		name            string
		authType        config.AuthType
		initialPassword string
		mockFunc        func(*gomock.Controller) *Handler
		wantErrMs       string
		wantCode        connect.Code
	}{
		{
			name:     "authType == None returns ErrFeatureNotAvailable",
			authType: config.AuthTypeNone,
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				uc := user_mock.NewMockusecase(ctrl)

				return New(uc, config.AuthTypeNone)
			},
			wantCode:  connect.CodeInvalidArgument,
			wantErrMs: "feature not available",
		},
		{
			name:            "authType == OIDC + initial_password not empty returns error",
			authType:        config.AuthTypeOIDC,
			initialPassword: "password",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				uc := user_mock.NewMockusecase(ctrl)

				return New(uc, config.AuthTypeOIDC)
			},
			wantCode:  connect.CodeInvalidArgument,
			wantErrMs: "initial_password must not be set in OIDC mode",
		},
		{
			name:            "authType == BasicAuth + success",
			authType:        config.AuthTypeBasicAuth,
			initialPassword: "password",
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					Create(gomock.Any(), email, "User", "password").
					Return(&domain.User{Email: email, Name: "User"}, nil)

				return New(uc, config.AuthTypeBasicAuth)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			_, err := h.CreateUser(t.Context(), connect.NewRequest(&userv1.CreateUserRequest{
				Email:           email,
				Name:            "User",
				InitialPassword: tt.initialPassword,
			}))

			if tt.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))
				if tt.wantErrMs != "" {
					assert.Contains(t, err.Error(), tt.wantErrMs)
				}

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestUserHandler_ResetUserPassword(t *testing.T) {
	t.Parallel()

	email := "user@example.com"

	tests := []struct {
		name     string
		authType config.AuthType
		mockFunc func(*gomock.Controller) *Handler
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name:     "authType == OIDC returns ErrFeatureNotAvailable",
			authType: config.AuthTypeOIDC,
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				uc := user_mock.NewMockusecase(ctrl)

				return New(uc, config.AuthTypeOIDC)
			},
			wantErr:  true,
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name:     "authType == BasicAuth: success",
			authType: config.AuthTypeBasicAuth,
			mockFunc: func(ctrl *gomock.Controller) *Handler {
				uc := user_mock.NewMockusecase(ctrl)
				uc.EXPECT().
					ResetPassword(gomock.Any(), email, "new-password").
					Return(nil)

				return New(uc, config.AuthTypeBasicAuth)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := tt.mockFunc(ctrl)

			_, err := h.ResetUserPassword(t.Context(), connect.NewRequest(&userv1.ResetUserPasswordRequest{
				Email:       email,
				NewPassword: "new-password",
			}))

			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
		})
	}
}
