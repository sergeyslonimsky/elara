package profile_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/profile"
)

func TestService_ChangePassword(t *testing.T) {
	t.Parallel()

	email := "user@example.com"
	currentPassword := "old-password"
	newPassword := "new-password"
	oldHash, _ := auth.HashPassword(currentPassword)

	// newUser returns a fresh *domain.User per sub-test. ChangePassword mutates
	// PasswordChangeRequired in-place; sharing a pointer across parallel
	// sub-tests would race.
	newUser := func() *domain.User {
		return &domain.User{Email: email, PasswordHash: oldHash}
	}

	tests := []struct {
		name     string
		currPass string
		mockFunc func(context.Context, *gomock.Controller) (*profile.Service, context.Context)
		errIs    error
		wantErr  string
		want     string
	}{
		{
			name:     "success with password verification",
			currPass: currentPassword,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: email, PasswordChangeRequired: false})
				svc, m := setupService(ctrl)

				m.users.EXPECT().Get(ctx, email).Return(newUser(), nil)
				m.pass.EXPECT().SetPassword(ctx, email, gomock.Any(), false).Return(nil)
				m.session.EXPECT().Create(gomock.Any()).Return("new-token", nil)

				return svc, ctx
			},
			want: "new-token",
		},
		{
			name:     "success skipping password verification (force change)",
			currPass: "any-password",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: email, PasswordChangeRequired: true})
				svc, m := setupService(ctrl)

				m.users.EXPECT().Get(ctx, email).Return(newUser(), nil)
				m.pass.EXPECT().SetPassword(ctx, email, gomock.Any(), false).Return(nil)
				m.session.EXPECT().Create(gomock.Any()).Return("new-token", nil)

				return svc, ctx
			},
			want: "new-token",
		},
		{
			name:     "unauthorized - no claims",
			currPass: currentPassword,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				return profile.New(nil, nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:     "unauthorized - wrong current password",
			currPass: "wrong-password",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: email, PasswordChangeRequired: false})
				svc, m := setupService(ctrl)

				m.users.EXPECT().Get(ctx, email).Return(newUser(), nil)

				return svc, ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:     "get user fails",
			currPass: currentPassword,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: email})
				svc, m := setupService(ctrl)

				m.users.EXPECT().Get(ctx, email).Return(nil, errors.New("db error"))

				return svc, ctx
			},
			wantErr: "get user: db error",
		},
		{
			name:     "set password fails",
			currPass: currentPassword,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: email, PasswordChangeRequired: true})
				svc, m := setupService(ctrl)

				m.users.EXPECT().Get(ctx, email).Return(newUser(), nil)
				m.pass.EXPECT().SetPassword(ctx, email, gomock.Any(), false).Return(errors.New("db error"))

				return svc, ctx
			},
			wantErr: "set password: db error",
		},
		{
			name:     "create session fails",
			currPass: currentPassword,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: email, PasswordChangeRequired: true})
				svc, m := setupService(ctrl)

				m.users.EXPECT().Get(ctx, email).Return(newUser(), nil)
				m.pass.EXPECT().SetPassword(ctx, email, gomock.Any(), false).Return(nil)
				m.session.EXPECT().Create(gomock.Any()).Return("", errors.New("session error"))

				return svc, ctx
			},
			wantErr: "create session: session error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.ChangePassword(ctx, tt.currPass, newPassword)

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
		})
	}
}
