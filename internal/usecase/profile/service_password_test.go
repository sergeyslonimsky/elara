package profile_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
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

	// newUser returns a fresh *domain.User per sub-test to avoid sharing a
	// pointer across parallel sub-tests.
	newUser := func() *domain.User {
		return &domain.User{Email: email, PasswordHash: oldHash}
	}

	tests := []struct {
		name     string
		params   profile.ChangePasswordParams
		mockFunc func(context.Context, *gomock.Controller) (*profile.Service, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success with password verification",
			params: profile.ChangePasswordParams{
				CurrentPassword: currentPassword,
				NewPassword:     newPassword,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth2.WithSession(
					ctx,
					&domain.Session{},
					&domain.User{Email: email, PasswordChangeRequired: false},
				)
				svc, m := setupService(ctrl)

				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.users.EXPECT().Get(gomock.Any(), email).Return(newUser(), nil)
				m.pass.EXPECT().SetPassword(gomock.Any(), email, gomock.Any(), false).Return(nil)
				m.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&domain.Session{ID: "new-s1"}, nil)

				return svc, ctx
			},
		},
		{
			name: "success skipping password verification (force change)",
			params: profile.ChangePasswordParams{
				CurrentPassword: "any-password",
				NewPassword:     newPassword,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth2.WithSession(
					ctx,
					&domain.Session{},
					&domain.User{Email: email, PasswordChangeRequired: true},
				)
				svc, m := setupService(ctrl)

				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.users.EXPECT().Get(gomock.Any(), email).Return(newUser(), nil)
				m.pass.EXPECT().SetPassword(gomock.Any(), email, gomock.Any(), false).Return(nil)
				m.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&domain.Session{ID: "new-s1"}, nil)

				return svc, ctx
			},
		},
		{
			name: "unauthorized - no user in context",
			params: profile.ChangePasswordParams{
				CurrentPassword: currentPassword,
				NewPassword:     newPassword,
			},
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*profile.Service, context.Context) {
				return profile.New(nil, nil, nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "unauthorized - wrong current password",
			params: profile.ChangePasswordParams{
				CurrentPassword: "wrong-password",
				NewPassword:     newPassword,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth2.WithSession(
					ctx,
					&domain.Session{},
					&domain.User{Email: email, PasswordChangeRequired: false},
				)
				svc, m := setupService(ctrl)

				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.users.EXPECT().Get(gomock.Any(), email).Return(newUser(), nil)

				return svc, ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "get user fails",
			params: profile.ChangePasswordParams{
				CurrentPassword: currentPassword,
				NewPassword:     newPassword,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth2.WithSession(ctx, &domain.Session{}, &domain.User{Email: email})
				svc, m := setupService(ctrl)

				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.users.EXPECT().Get(gomock.Any(), email).Return(nil, errors.New("db error"))

				return svc, ctx
			},
			wantErr: "get user: db error",
		},
		{
			name: "set password fails",
			params: profile.ChangePasswordParams{
				CurrentPassword: currentPassword,
				NewPassword:     newPassword,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth2.WithSession(
					ctx,
					&domain.Session{},
					&domain.User{Email: email, PasswordChangeRequired: true},
				)
				svc, m := setupService(ctrl)

				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.users.EXPECT().Get(gomock.Any(), email).Return(newUser(), nil)
				m.pass.EXPECT().
					SetPassword(gomock.Any(), email, gomock.Any(), false).
					Return(errors.New("db error"))

				return svc, ctx
			},
			wantErr: "set password: db error",
		},
		{
			name: "create session fails",
			params: profile.ChangePasswordParams{
				CurrentPassword: currentPassword,
				NewPassword:     newPassword,
			},
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth2.WithSession(
					ctx,
					&domain.Session{},
					&domain.User{Email: email, PasswordChangeRequired: true},
				)
				svc, m := setupService(ctrl)

				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					},
				)
				m.users.EXPECT().Get(gomock.Any(), email).Return(newUser(), nil)
				m.pass.EXPECT().SetPassword(gomock.Any(), email, gomock.Any(), false).Return(nil)
				m.sessions.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("session error"))

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

			got, err := sut.ChangePassword(ctx, tt.params)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, "new-s1", got.ID)
		})
	}
}
