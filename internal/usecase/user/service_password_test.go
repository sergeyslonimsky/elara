package user_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/usecase/user"
)

func TestService_ResetPassword(t *testing.T) {
	t.Parallel()

	adminEmail := "admin@example.com"
	targetEmail := "user@example.com"
	newPassword := "reset-password"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*user.Service, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().
					SetPassword(ctx, targetEmail, gomock.Any(), true).
					Return(nil)

				return sut, ctx
			},
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*user.Service, context.Context) {
				return user.New(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(false, nil)

				return sut, ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "enforce fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(false, assert.AnError)

				return sut, ctx
			},
			wantErr: "enforce",
		},
		{
			name: "set password fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().
					SetPassword(ctx, targetEmail, gomock.Any(), true).
					Return(assert.AnError)

				return sut, ctx
			},
			wantErr: "set password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.ResetPassword(ctx, targetEmail, newPassword)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}
