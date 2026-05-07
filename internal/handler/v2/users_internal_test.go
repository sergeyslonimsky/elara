package v2

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestUserHandler_ListUsers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		users   []*domain.User
		repoErr error
		wantLen int
		wantErr bool
	}{
		{
			name:    "returns all users",
			users:   []*domain.User{{Email: "a@example.com"}, {Email: "b@example.com"}},
			wantLen: 2,
		},
		{
			name:    "returns empty list",
			users:   []*domain.User{},
			wantLen: 0,
		},
		{
			name:    "storage error returns internal",
			repoErr: errors.New("db error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			lister := auth_mock.NewMockuserLister(ctrl)
			lister.EXPECT().List(gomock.Any()).Return(tc.users, tc.repoErr)

			h := NewUserHandler(
				authuc.NewListUsersUseCase(allowAllClientsHandlerEnforcer{}, lister),
				nil,
				nil,
				nil,
				nil,
				config.AuthTypeOIDC,
			)

			resp, err := h.ListUsers(clientsHandlerTestCtx(), connect.NewRequest(&authv1.ListUsersRequest{}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetUsers(), tc.wantLen)
		})
	}
}

func TestUserHandler_GetUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		email    string
		user     *domain.User
		repoErr  error
		wantErr  bool
		wantCode connect.Code
	}{
		{
			name:  "returns user by email",
			email: "alice@example.com",
			user:  &domain.User{Email: "alice@example.com", Name: "Alice"},
		},
		{
			name:     "not found returns NotFound code",
			email:    "ghost@example.com",
			repoErr:  domain.NewNotFoundError("user", "ghost@example.com"),
			wantErr:  true,
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMockuserGetter(ctrl)
			getter.EXPECT().Get(gomock.Any(), tc.email).Return(tc.user, tc.repoErr)

			h := NewUserHandler(
				nil,
				authuc.NewGetUserUseCase(allowAllClientsHandlerEnforcer{}, getter),
				nil,
				nil,
				nil,
				config.AuthTypeOIDC,
			)

			resp, err := h.GetUser(clientsHandlerTestCtx(), connect.NewRequest(&authv1.GetUserRequest{Email: tc.email}))

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.email, resp.Msg.GetUser().GetEmail())
		})
	}
}

func TestUserHandler_DeleteUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		authType  config.AuthType
		mockFunc  func(*gomock.Controller) *authuc.DeleteUserUseCase
		wantErr   bool
		wantErrMs string
		wantCode  connect.Code
	}{
		{
			name:      "authType == OIDC returns ErrFeatureNotAvailable",
			authType:  config.AuthTypeOIDC,
			wantErr:   true,
			wantCode:  connect.CodeInvalidArgument,
			wantErrMs: "feature not available",
		},
		{
			name:      "authType == None returns ErrFeatureNotAvailable",
			authType:  config.AuthTypeNone,
			wantErr:   true,
			wantCode:  connect.CodeInvalidArgument,
			wantErrMs: "feature not available",
		},
		{
			name:     "authType == BasicAuth: guard passes, usecase returns error",
			authType: config.AuthTypeBasicAuth,
			mockFunc: func(ctrl *gomock.Controller) *authuc.DeleteUserUseCase {
				enforcer := auth_mock.NewMockdeleteUserEnforcer(ctrl)
				users := auth_mock.NewMockuserGetterDeleter(ctrl)

				enforcer.EXPECT().
					Enforce(gomock.Any(), auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				users.EXPECT().Get(gomock.Any(), "notfound@example.com").Return(nil, domain.ErrNotFound)

				return authuc.NewDeleteUserUseCase(enforcer, users)
			},
			wantErr:  true,
			wantCode: connect.CodeNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			var deleteUC *authuc.DeleteUserUseCase
			if tc.mockFunc != nil {
				deleteUC = tc.mockFunc(ctrl)
			}

			h := NewUserHandler(nil, nil, nil, nil, deleteUC, tc.authType)

			_, err := h.DeleteUser(
				clientsHandlerTestCtx(),
				connect.NewRequest(&authv1.DeleteUserRequest{Email: "notfound@example.com"}),
			)

			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))
				if tc.wantErrMs != "" {
					assert.Contains(t, err.Error(), tc.wantErrMs)
				}

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestUserHandler_CreateUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		authType        config.AuthType
		initialPassword string
		mockFunc        func(*gomock.Controller) *authuc.CreateUserUseCase
		wantErrMs       string
		wantCode        connect.Code
	}{
		{
			name:      "authType == None returns ErrFeatureNotAvailable",
			authType:  config.AuthTypeNone,
			wantCode:  connect.CodeInvalidArgument,
			wantErrMs: "feature not available",
		},
		{
			name:            "authType == OIDC + initial_password not empty returns error",
			authType:        config.AuthTypeOIDC,
			initialPassword: "password",
			wantCode:        connect.CodeInvalidArgument,
			wantErrMs:       "initial_password must not be set in OIDC mode",
		},
		{
			name:            "authType == BasicAuth + initial_password empty returns error",
			authType:        config.AuthTypeBasicAuth,
			initialPassword: "",
			wantCode:        connect.CodeInvalidArgument,
			wantErrMs:       "initial_password is required in basic-auth mode",
		},
		{
			name:            "authType == OIDC + initial_password empty, usecase returns error",
			authType:        config.AuthTypeOIDC,
			initialPassword: "",
			mockFunc: func(ctrl *gomock.Controller) *authuc.CreateUserUseCase {
				enforcer := auth_mock.NewMockcreateUserEnforcer(ctrl)
				users := auth_mock.NewMockuserCreator(ctrl)

				enforcer.EXPECT().
					Enforce(gomock.Any(), auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				users.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(domain.ErrNotFound)

				return authuc.NewCreateUserUseCase(enforcer, users)
			},
			wantCode: connect.CodeNotFound,
		},
		{
			name:            "authType == BasicAuth + initial_password not empty, usecase returns error",
			authType:        config.AuthTypeBasicAuth,
			initialPassword: "password",
			mockFunc: func(ctrl *gomock.Controller) *authuc.CreateUserUseCase {
				enforcer := auth_mock.NewMockcreateUserEnforcer(ctrl)
				users := auth_mock.NewMockuserCreator(ctrl)

				enforcer.EXPECT().
					Enforce(gomock.Any(), auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				users.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(domain.ErrAlreadyExists)

				return authuc.NewCreateUserUseCase(enforcer, users)
			},
			wantCode: connect.CodeAlreadyExists,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			var createUC *authuc.CreateUserUseCase
			if tc.mockFunc != nil {
				createUC = tc.mockFunc(ctrl)
			}

			h := NewUserHandler(nil, nil, createUC, nil, nil, tc.authType)

			_, err := h.CreateUser(
				clientsHandlerTestCtx(),
				connect.NewRequest(&authv1.CreateUserRequest{
					Email:           "user@example.com",
					Name:            "User",
					InitialPassword: tc.initialPassword,
				}),
			)

			if tc.wantCode != 0 {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, connect.CodeOf(err))
				if tc.wantErrMs != "" {
					assert.Contains(t, err.Error(), tc.wantErrMs)
				}

				return
			}

			require.NoError(t, err)
		})
	}
}
