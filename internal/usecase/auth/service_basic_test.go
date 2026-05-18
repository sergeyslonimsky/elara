package auth_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

func TestService_BasicLogin(t *testing.T) {
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
		name     string
		email    string
		password string
		mockFunc func(*gomock.Controller) *authuc.Service
		errIs    error
		wantErr  string
		want     string
		wantUser *domain.User
	}{
		{
			name:     "success",
			email:    user.Email,
			password: password,
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.users.EXPECT().Get(gomock.Any(), user.Email).Return(user, nil)
				m.session.EXPECT().Create(user).Return("token123", nil)

				return svc
			},
			want:     "token123",
			wantUser: user,
		},
		{
			name:     "success admin bootstrap",
			email:    adminEmail,
			password: password,
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.users.EXPECT().Get(gomock.Any(), adminEmail).Return(adminUser, nil)
				m.admin.EXPECT().EnsureMember(gomock.Any(), adminEmail).Return(nil)
				m.session.EXPECT().Create(adminUser).Return("admin-token", nil)

				return svc
			},
			want:     "admin-token",
			wantUser: adminUser,
		},
		{
			name:     "user not found",
			email:    "unknown@example.com",
			password: password,
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.users.EXPECT().Get(gomock.Any(), "unknown@example.com").Return(nil, domain.ErrNotFound)

				return svc
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:     "wrong password",
			email:    user.Email,
			password: "wrong-password",
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.users.EXPECT().Get(gomock.Any(), user.Email).Return(user, nil)

				return svc
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:     "session creation fails",
			email:    user.Email,
			password: password,
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.users.EXPECT().Get(gomock.Any(), user.Email).Return(user, nil)
				m.session.EXPECT().Create(user).Return("", errors.New("session error"))

				return svc
			},
			wantErr: "create session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			got, u, err := sut.BasicLogin(t.Context(), tt.email, tt.password)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantUser, u)
		})
	}
}
