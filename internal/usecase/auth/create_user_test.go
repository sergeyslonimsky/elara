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

func TestCreateUserUseCase_Execute(t *testing.T) {
	t.Parallel()

	adminEmail := "admin@example.com"
	email := "new-user@example.com"
	name := "New User"
	password := "initial-password"

	tests := []struct {
		name     string
		email    string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.CreateUserUseCase, context.Context)
		errIs    error
		wantErr  string
		want     *domain.User
	}{
		{
			name:  "success",
			email: email,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.CreateUserUseCase, context.Context) {
				claims := &auth.Claims{Email: adminEmail}
				ctx = auth.WithClaims(ctx, claims)

				mockEnforcer := mock_auth.NewMockcreateUserEnforcer(ctrl)
				mockUsers := mock_auth.NewMockuserCreator(ctrl)

				mockEnforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				mockUsers.EXPECT().Upsert(ctx, gomock.AssignableToTypeOf(&domain.User{})).Return(nil)
				mockUsers.EXPECT().SetPassword(ctx, email, gomock.Any(), true).Return(nil)

				return authuc.NewCreateUserUseCase(mockEnforcer, mockUsers), ctx
			},
			want: &domain.User{
				Email:    email,
				Name:     name,
				Provider: domain.ProviderBasicAuth,
			},
		},
		{
			name:  "validation error",
			email: "invalid-email",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.CreateUserUseCase, context.Context) {
				claims := &auth.Claims{Email: adminEmail}
				ctx = auth.WithClaims(ctx, claims)

				mockEnforcer := mock_auth.NewMockcreateUserEnforcer(ctrl)
				mockEnforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)

				return authuc.NewCreateUserUseCase(mockEnforcer, nil), ctx
			},
			wantErr: "validate user",
		},
		{
			name:  "forbidden",
			email: email,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.CreateUserUseCase, context.Context) {
				claims := &auth.Claims{Email: "user@example.com"}
				ctx = auth.WithClaims(ctx, claims)

				mockEnforcer := mock_auth.NewMockcreateUserEnforcer(ctrl)
				mockEnforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(false, nil)

				return authuc.NewCreateUserUseCase(mockEnforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name:  "unauthorized",
			email: email,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.CreateUserUseCase, context.Context) {
				return authuc.NewCreateUserUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:  "upsert fails",
			email: email,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.CreateUserUseCase, context.Context) {
				claims := &auth.Claims{Email: adminEmail}
				ctx = auth.WithClaims(ctx, claims)

				mockEnforcer := mock_auth.NewMockcreateUserEnforcer(ctrl)
				mockUsers := mock_auth.NewMockuserCreator(ctrl)

				mockEnforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				mockUsers.EXPECT().Upsert(ctx, gomock.Any()).Return(assert.AnError)

				return authuc.NewCreateUserUseCase(mockEnforcer, mockUsers), ctx
			},
			wantErr: "upsert user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Execute(ctx, tt.email, name, password)

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
