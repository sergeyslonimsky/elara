package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	mock_casbin "github.com/sergeyslonimsky/elara/internal/auth/casbin/mocks"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	mock_auth "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestBasicLoginUseCase_Execute(t *testing.T) {
	t.Parallel()

	password := "password123"
	hash, _ := auth.HashPassword(password)
	user := &domain.User{
		Email:        "user@example.com",
		PasswordHash: hash,
	}
	adminEmail := "admin@example.com"
	adminHash, _ := auth.HashPassword(password)
	adminUser := &domain.User{
		Email:        adminEmail,
		PasswordHash: adminHash,
	}

	tests := []struct {
		name      string
		email     string
		password  string
		mockFunc  func(*gomock.Controller) *authuc.BasicLoginUseCase
		errIs     error
		wantErr   string
		wantToken string
		wantUser  *domain.User
	}{
		{
			name:     "success",
			email:    user.Email,
			password: password,
			mockFunc: func(ctrl *gomock.Controller) *authuc.BasicLoginUseCase {
				mockUsers := mock_auth.NewMockbasicAuthUserGetter(ctrl)
				mockSession := mock_auth.NewMocksessionCreator(ctrl)

				mockUsers.EXPECT().Get(gomock.Any(), user.Email).Return(user, nil)
				mockSession.EXPECT().Create(user).Return("token123", nil)

				return authuc.NewBasicLoginUseCase(mockUsers, mockSession, nil, adminEmail)
			},
			wantToken: "token123",
			wantUser:  user,
		},
		{
			name:     "success admin bootstrap",
			email:    adminEmail,
			password: password,
			mockFunc: func(ctrl *gomock.Controller) *authuc.BasicLoginUseCase {
				mockUsers := mock_auth.NewMockbasicAuthUserGetter(ctrl)
				mockSession := mock_auth.NewMocksessionCreator(ctrl)
				mockEnforcer := mock_casbin.NewMockBootstrapEnforcer(ctrl)

				mockUsers.EXPECT().Get(gomock.Any(), adminEmail).Return(adminUser, nil)
				mockEnforcer.EXPECT().GetRolesForUser(adminEmail, auth.ObjectAll).Return([]string{}, nil)
				mockEnforcer.EXPECT().AddRoleForUser(adminEmail, auth.RoleAdmin, auth.ObjectAll).Return(nil)
				mockSession.EXPECT().Create(adminUser).Return("admin-token", nil)

				return authuc.NewBasicLoginUseCase(mockUsers, mockSession, mockEnforcer, adminEmail)
			},
			wantToken: "admin-token",
			wantUser:  adminUser,
		},
		{
			name:     "user not found",
			email:    "unknown@example.com",
			password: password,
			mockFunc: func(ctrl *gomock.Controller) *authuc.BasicLoginUseCase {
				mockUsers := mock_auth.NewMockbasicAuthUserGetter(ctrl)
				mockUsers.EXPECT().Get(gomock.Any(), "unknown@example.com").Return(nil, domain.ErrNotFound)

				return authuc.NewBasicLoginUseCase(mockUsers, nil, nil, adminEmail)
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:     "wrong password",
			email:    user.Email,
			password: "wrong-password",
			mockFunc: func(ctrl *gomock.Controller) *authuc.BasicLoginUseCase {
				mockUsers := mock_auth.NewMockbasicAuthUserGetter(ctrl)
				mockUsers.EXPECT().Get(gomock.Any(), user.Email).Return(user, nil)

				return authuc.NewBasicLoginUseCase(mockUsers, nil, nil, adminEmail)
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:     "session creation fails",
			email:    user.Email,
			password: password,
			mockFunc: func(ctrl *gomock.Controller) *authuc.BasicLoginUseCase {
				mockUsers := mock_auth.NewMockbasicAuthUserGetter(ctrl)
				mockSession := mock_auth.NewMocksessionCreator(ctrl)

				mockUsers.EXPECT().Get(gomock.Any(), user.Email).Return(user, nil)
				mockSession.EXPECT().Create(user).Return("", assert.AnError)

				return authuc.NewBasicLoginUseCase(mockUsers, mockSession, nil, adminEmail)
			},
			wantErr: "create session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := tt.mockFunc(ctrl)

			token, u, err := uc.Execute(t.Context(), tt.email, tt.password)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, token)
			assert.Equal(t, tt.wantUser, u)
		})
	}
}
