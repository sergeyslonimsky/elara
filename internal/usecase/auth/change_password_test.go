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

func TestChangePasswordUseCase_Execute(t *testing.T) {
	t.Parallel()

	email := "user@example.com"
	currentPassword := "old-password"
	newPassword := "new-password"
	oldHash, _ := auth.HashPassword(currentPassword)
	user := &domain.User{
		Email:        email,
		PasswordHash: oldHash,
	}

	tests := []struct {
		name     string
		currPass string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.ChangePasswordUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name:     "success with password verification",
			currPass: currentPassword,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ChangePasswordUseCase, context.Context) {
				claims := &auth.Claims{Email: email, PasswordChangeRequired: false}
				ctx = auth.WithClaims(ctx, claims)

				mockReader := mock_auth.NewMockpasswordReader(ctrl)
				mockWriter := mock_auth.NewMockpasswordWriter(ctrl)
				mockSession := mock_auth.NewMocksessionCreator(ctrl)

				mockReader.EXPECT().Get(ctx, email).Return(user, nil)
				mockWriter.EXPECT().SetPassword(ctx, email, gomock.Any(), false).Return(nil)
				mockSession.EXPECT().Create(gomock.Any()).Return("new-token", nil)

				return authuc.NewChangePasswordUseCase(mockReader, mockWriter, mockSession), ctx
			},
		},
		{
			name:     "success skipping password verification (force change)",
			currPass: "any-password",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ChangePasswordUseCase, context.Context) {
				claims := &auth.Claims{Email: email, PasswordChangeRequired: true}
				ctx = auth.WithClaims(ctx, claims)

				mockReader := mock_auth.NewMockpasswordReader(ctrl)
				mockWriter := mock_auth.NewMockpasswordWriter(ctrl)
				mockSession := mock_auth.NewMocksessionCreator(ctrl)

				mockReader.EXPECT().Get(ctx, email).Return(user, nil)
				mockWriter.EXPECT().SetPassword(ctx, email, gomock.Any(), false).Return(nil)
				mockSession.EXPECT().Create(gomock.Any()).Return("new-token", nil)

				return authuc.NewChangePasswordUseCase(mockReader, mockWriter, mockSession), ctx
			},
		},
		{
			name:     "unauthorized - no claims",
			currPass: currentPassword,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ChangePasswordUseCase, context.Context) {
				return authuc.NewChangePasswordUseCase(nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:     "unauthorized - wrong current password",
			currPass: "wrong-password",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ChangePasswordUseCase, context.Context) {
				claims := &auth.Claims{Email: email, PasswordChangeRequired: false}
				ctx = auth.WithClaims(ctx, claims)

				mockReader := mock_auth.NewMockpasswordReader(ctrl)
				mockReader.EXPECT().Get(ctx, email).Return(user, nil)

				return authuc.NewChangePasswordUseCase(mockReader, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:     "get user fails",
			currPass: currentPassword,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ChangePasswordUseCase, context.Context) {
				claims := &auth.Claims{Email: email}
				ctx = auth.WithClaims(ctx, claims)

				mockReader := mock_auth.NewMockpasswordReader(ctrl)
				mockReader.EXPECT().Get(ctx, email).Return(nil, assert.AnError)

				return authuc.NewChangePasswordUseCase(mockReader, nil, nil), ctx
			},
			wantErr: "get user",
		},
		{
			name:     "set password fails",
			currPass: currentPassword,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.ChangePasswordUseCase, context.Context) {
				claims := &auth.Claims{Email: email, PasswordChangeRequired: true}
				ctx = auth.WithClaims(ctx, claims)

				mockReader := mock_auth.NewMockpasswordReader(ctrl)
				mockWriter := mock_auth.NewMockpasswordWriter(ctrl)

				mockReader.EXPECT().Get(ctx, email).Return(user, nil)
				mockWriter.EXPECT().SetPassword(ctx, email, gomock.Any(), false).Return(assert.AnError)

				return authuc.NewChangePasswordUseCase(mockReader, mockWriter, nil), ctx
			},
			wantErr: "set password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			_, err := sut.Execute(ctx, tt.currPass, newPassword)

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
