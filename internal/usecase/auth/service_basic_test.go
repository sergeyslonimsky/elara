package auth_test

import (
	"context"
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
		Status:       domain.UserStatusActive,
	}

	tests := []struct {
		name     string
		params   authuc.LoginParams
		mockFunc func(*gomock.Controller) *authuc.Service
		errIs    error
		wantErr  string
		wantUser *domain.User
	}{
		{
			name: "success",
			params: authuc.LoginParams{
				Email:    user.Email,
				Password: password,
			},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.users.EXPECT().GetByIdentity(gomock.Any(), string(domain.ProviderBasic), user.Email).Return(user, nil)

				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&domain.Session{ID: "new-s1"}, nil)

				return svc
			},
			wantUser: user,
		},
		{
			name: "user not found",
			params: authuc.LoginParams{
				Email:    "unknown@example.com",
				Password: password,
			},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.users.EXPECT().
					GetByIdentity(gomock.Any(), string(domain.ProviderBasic), "unknown@example.com").
					Return(nil, domain.ErrNotFound)

				return svc
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "wrong password",
			params: authuc.LoginParams{
				Email:    user.Email,
				Password: "wrong-password",
			},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.users.EXPECT().GetByIdentity(gomock.Any(), string(domain.ProviderBasic), user.Email).Return(user, nil)

				return svc
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "deactivated user",
			params: authuc.LoginParams{
				Email:    user.Email,
				Password: password,
			},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				deactUser := &domain.User{
					Email:        user.Email,
					PasswordHash: user.PasswordHash,
					Status:       domain.UserStatusDeactivated,
				}
				m.users.EXPECT().
					GetByIdentity(gomock.Any(), string(domain.ProviderBasic), user.Email).
					Return(deactUser, nil)

				return svc
			},
			errIs: domain.ErrUserDeactivated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			u, sess, err := sut.BasicLogin(t.Context(), tt.params)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantUser, u)
			require.NotNil(t, sess)
			assert.Equal(t, "new-s1", sess.ID)
		})
	}
}
