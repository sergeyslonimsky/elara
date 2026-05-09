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

func TestService_Callback(t *testing.T) {
	t.Parallel()

	identity := &auth.Identity{
		Email:   "user@example.com",
		Name:    "Test User",
		Picture: "https://example.com/pic.png",
	}

	tests := []struct {
		name     string
		code     string
		nonce    string
		mockFunc func(*gomock.Controller) *authuc.Service
		errIs    error
		wantErr  string
		wantUser *domain.User
		want     string
	}{
		{
			name:  "success",
			code:  "auth-code",
			nonce: "test-nonce",
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.provider.EXPECT().Exchange(gomock.Any(), "auth-code", "test-nonce").Return(identity, nil)
				m.users.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
				m.session.EXPECT().Create(gomock.Any()).Return("token123", nil)

				return svc
			},
			want: "token123",
			wantUser: &domain.User{
				Email:    identity.Email,
				Name:     identity.Name,
				Picture:  identity.Picture,
				Provider: domain.ProviderOIDC,
			},
		},
		{
			name:  "success admin bootstrap",
			code:  "auth-code",
			nonce: "test-nonce",
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				// admin email is "admin@example.com" in setupService
				adminIdentity := &auth.Identity{Email: "admin@example.com", Name: "Admin"}
				m.provider.EXPECT().Exchange(gomock.Any(), "auth-code", "test-nonce").Return(adminIdentity, nil)
				m.users.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
				m.enforcer.EXPECT().GetRolesForUser("admin@example.com", auth.ObjectAll).Return([]string{}, nil)
				m.enforcer.EXPECT().AddRoleForUser("admin@example.com", auth.RoleAdmin, auth.ObjectAll).Return(nil)
				m.session.EXPECT().Create(gomock.Any()).Return("admin-token", nil)

				return svc
			},
			want: "admin-token",
		},
		{
			name:  "exchange error",
			code:  "bad-code",
			nonce: "test-nonce",
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.provider.EXPECT().
					Exchange(gomock.Any(), "bad-code", "test-nonce").
					Return(nil, errors.New("exchange failed"))

				return svc
			},
			wantErr: "exchange code",
		},
		{
			name:  "upsert error",
			code:  "auth-code",
			nonce: "test-nonce",
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.provider.EXPECT().Exchange(gomock.Any(), "auth-code", "test-nonce").Return(identity, nil)
				m.users.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

				return svc
			},
			wantErr: "upsert user",
		},
		{
			name:  "session creation error",
			code:  "auth-code",
			nonce: "test-nonce",
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.provider.EXPECT().Exchange(gomock.Any(), "auth-code", "test-nonce").Return(identity, nil)
				m.users.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
				m.session.EXPECT().Create(gomock.Any()).Return("", errors.New("session error"))

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

			got, user, err := sut.Callback(t.Context(), tt.code, tt.nonce)

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
			if tt.wantUser != nil {
				assert.Equal(t, tt.wantUser.Email, user.Email)
				assert.Equal(t, tt.wantUser.Name, user.Name)
				assert.Equal(t, tt.wantUser.Picture, user.Picture)
				assert.Equal(t, tt.wantUser.Provider, user.Provider)
			}
		})
	}
}
