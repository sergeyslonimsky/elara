package auth_test

import (
	"context"
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
		params   authuc.CallbackParams
		mockFunc func(*gomock.Controller) *authuc.Service
		errIs    error
		wantErr  string
		wantUser *domain.User
	}{
		{
			name: "success",
			params: authuc.CallbackParams{
				Code:  "auth-code",
				Nonce: "test-nonce",
			},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.provider.EXPECT().
					Exchange(gomock.Any(), "auth-code", "test-nonce").
					Return(identity, nil)

				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.users.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
				m.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&domain.Session{ID: "new-s1"}, nil)

				return svc
			},
			wantUser: &domain.User{
				Email:    identity.Email,
				Name:     identity.Name,
				Picture:  identity.Picture,
				Provider: domain.ProviderOIDC,
			},
		},
		{
			name: "success admin bootstrap",
			params: authuc.CallbackParams{
				Code:  "auth-code",
				Nonce: "test-nonce",
			},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				// admin email is "admin@example.com" in setupService
				adminIdentity := &auth.Identity{Email: "admin@example.com", Name: "Admin"}
				m.provider.EXPECT().
					Exchange(gomock.Any(), "auth-code", "test-nonce").
					Return(adminIdentity, nil)

				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.users.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(nil)
				m.admin.EXPECT().EnsureMember(gomock.Any(), "admin@example.com").Return(nil)
				m.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&domain.Session{ID: "new-s1"}, nil)

				return svc
			},
		},
		{
			name: "exchange error",
			params: authuc.CallbackParams{
				Code:  "bad-code",
				Nonce: "test-nonce",
			},
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
			name: "upsert error",
			params: authuc.CallbackParams{
				Code:  "auth-code",
				Nonce: "test-nonce",
			},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.provider.EXPECT().
					Exchange(gomock.Any(), "auth-code", "test-nonce").
					Return(identity, nil)

				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.users.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

				return svc
			},
			wantErr: "upsert user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			user, sess, err := sut.Callback(t.Context(), tt.params)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			require.NotNil(t, sess)
			assert.Equal(t, "new-s1", sess.ID)

			if tt.wantUser != nil {
				assert.Equal(t, tt.wantUser.Email, user.Email)
				assert.Equal(t, tt.wantUser.Name, user.Name)
				assert.Equal(t, tt.wantUser.Picture, user.Picture)
				assert.Equal(t, tt.wantUser.Provider, user.Provider)
			}
		})
	}
}
