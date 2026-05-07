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
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestDeleteUserUseCase_Execute(t *testing.T) {
	t.Parallel()

	adminEmail := "admin@example.com"
	targetEmail := "target@example.com"

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authuc.DeleteUserUseCase, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "no claims in context",
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*authuc.DeleteUserUseCase, context.Context) {
				return authuc.NewDeleteUserUseCase(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "caller not admin",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteUserUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				enforcer := auth_mock.NewMockdeleteUserEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce("user@example.com", auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(false, nil)

				return authuc.NewDeleteUserUseCase(enforcer, nil), ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "self-deletion",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteUserUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: targetEmail})
				enforcer := auth_mock.NewMockdeleteUserEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce(targetEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)

				return authuc.NewDeleteUserUseCase(enforcer, nil), ctx
			},
			wantErr: "cannot delete your own account",
		},
		{
			name: "user not found",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteUserUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockdeleteUserEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				users := auth_mock.NewMockuserGetterDeleter(ctrl)
				users.EXPECT().Get(ctx, targetEmail).Return(nil, domain.ErrNotFound)

				return authuc.NewDeleteUserUseCase(enforcer, users), ctx
			},
			errIs: domain.ErrNotFound,
		},
		{
			name: "last admin guard",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteUserUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockdeleteUserEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				users := auth_mock.NewMockuserGetterDeleter(ctrl)
				users.EXPECT().Get(ctx, targetEmail).Return(&domain.User{Email: targetEmail}, nil)
				enforcer.EXPECT().GetGroupingPolicy().Return([][]string{{targetEmail, auth.RoleAdmin, auth.ObjectAll}})

				return authuc.NewDeleteUserUseCase(enforcer, users), ctx
			},
			wantErr: "cannot delete the last admin",
		},
		{
			name: "target is admin but not the last one",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteUserUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockdeleteUserEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				users := auth_mock.NewMockuserGetterDeleter(ctrl)
				users.EXPECT().Get(ctx, targetEmail).Return(&domain.User{Email: targetEmail}, nil)
				enforcer.EXPECT().GetGroupingPolicy().Return([][]string{
					{targetEmail, auth.RoleAdmin, auth.ObjectAll},
					{adminEmail, auth.RoleAdmin, auth.ObjectAll},
				})
				users.EXPECT().Delete(ctx, targetEmail).Return(nil)
				enforcer.EXPECT().DeleteUser(targetEmail).Return(nil)

				return authuc.NewDeleteUserUseCase(enforcer, users), ctx
			},
		},
		{
			name: "happy path (target is not admin)",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteUserUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockdeleteUserEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				users := auth_mock.NewMockuserGetterDeleter(ctrl)
				users.EXPECT().Get(ctx, targetEmail).Return(&domain.User{Email: targetEmail}, nil)
				enforcer.EXPECT().GetGroupingPolicy().Return([][]string{
					{adminEmail, auth.RoleAdmin, auth.ObjectAll},
				})
				users.EXPECT().Delete(ctx, targetEmail).Return(nil)
				enforcer.EXPECT().DeleteUser(targetEmail).Return(nil)

				return authuc.NewDeleteUserUseCase(enforcer, users), ctx
			},
		},
		{
			name: "Delete (from users) returns error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteUserUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockdeleteUserEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				users := auth_mock.NewMockuserGetterDeleter(ctrl)
				users.EXPECT().Get(ctx, targetEmail).Return(&domain.User{Email: targetEmail}, nil)
				enforcer.EXPECT().GetGroupingPolicy().Return([][]string{
					{adminEmail, auth.RoleAdmin, auth.ObjectAll},
				})
				users.EXPECT().Delete(ctx, targetEmail).Return(assert.AnError)

				return authuc.NewDeleteUserUseCase(enforcer, users), ctx
			},
			wantErr: assert.AnError.Error(),
		},
		{
			name: "DeleteUser (Casbin) returns error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authuc.DeleteUserUseCase, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				enforcer := auth_mock.NewMockdeleteUserEnforcer(ctrl)
				enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				users := auth_mock.NewMockuserGetterDeleter(ctrl)
				users.EXPECT().Get(ctx, targetEmail).Return(&domain.User{Email: targetEmail}, nil)
				enforcer.EXPECT().GetGroupingPolicy().Return([][]string{
					{adminEmail, auth.RoleAdmin, auth.ObjectAll},
				})
				users.EXPECT().Delete(ctx, targetEmail).Return(nil)
				enforcer.EXPECT().DeleteUser(targetEmail).Return(assert.AnError)

				return authuc.NewDeleteUserUseCase(enforcer, users), ctx
			},
			wantErr: assert.AnError.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Execute(ctx, targetEmail)

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
