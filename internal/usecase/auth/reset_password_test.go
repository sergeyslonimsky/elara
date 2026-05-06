package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	mock_auth "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestResetPasswordUseCase_Execute(t *testing.T) {
	t.Parallel()

	adminEmail := "admin@example.com"
	targetEmail := "user@example.com"
	newPassword := "reset-password"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.ResetPasswordUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ResetPasswordUseCase, context.Context) {
				claims := &auth.Claims{Email: adminEmail}
				ctx = auth.WithClaims(ctx, claims)

				mockEnforcer := mock_auth.NewMockresetPasswordEnforcer(ctrl)
				mockWriter := mock_auth.NewMockpasswordWriter(ctrl)

				mockEnforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				mockWriter.EXPECT().
					SetPassword(ctx, targetEmail, gomock.Any(), true).
					Return(nil)

				return authuc.NewResetPasswordUseCase(mockEnforcer, mockWriter), ctx
			},
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ResetPasswordUseCase, context.Context) {
				claims := &auth.Claims{Email: "not-admin@example.com"}
				ctx = auth.WithClaims(ctx, claims)

				mockEnforcer := mock_auth.NewMockresetPasswordEnforcer(ctrl)
				mockEnforcer.EXPECT().
					Enforce("not-admin@example.com", auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(false, nil)

				return authuc.NewResetPasswordUseCase(mockEnforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "unauthorized",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ResetPasswordUseCase, context.Context) {
				return authuc.NewResetPasswordUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "enforce fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ResetPasswordUseCase, context.Context) {
				claims := &auth.Claims{Email: adminEmail}
				ctx = auth.WithClaims(ctx, claims)

				mockEnforcer := mock_auth.NewMockresetPasswordEnforcer(ctrl)
				mockEnforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(false, assert.AnError)

				return authuc.NewResetPasswordUseCase(mockEnforcer, nil), ctx
			},
			wantErr: "enforce",
		},
		{
			name: "set password fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ResetPasswordUseCase, context.Context) {
				claims := &auth.Claims{Email: adminEmail}
				ctx = auth.WithClaims(ctx, claims)

				mockEnforcer := mock_auth.NewMockresetPasswordEnforcer(ctrl)
				mockWriter := mock_auth.NewMockpasswordWriter(ctrl)

				mockEnforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				mockWriter.EXPECT().
					SetPassword(ctx, targetEmail, gomock.Any(), true).
					Return(assert.AnError)

				return authuc.NewResetPasswordUseCase(mockEnforcer, mockWriter), ctx
			},
			wantErr: "set password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Execute(ctx, targetEmail, newPassword)

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
