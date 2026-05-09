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

func TestService_Create(t *testing.T) {
	t.Parallel()

	adminEmail := "admin@example.com"
	email := "new-user@example.com"
	name := "New User"
	password := "initial-password"

	tests := []struct {
		name     string
		email    string
		password string
		mockFunc func(context.Context, *gomock.Controller) (*user.Service, context.Context)
		errIs    error
		wantErr  string
		want     *domain.User
	}{
		{
			name:     "success",
			email:    email,
			password: password,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().Upsert(ctx, gomock.AssignableToTypeOf(&domain.User{})).Return(nil)
				m.store.EXPECT().SetPassword(ctx, email, gomock.Any(), true).Return(nil)

				return sut, ctx
			},
			want: &domain.User{
				Email:    email,
				Name:     name,
				Provider: domain.ProviderBasicAuth,
			},
		},
		{
			name:     "OIDC pre-create (empty password)",
			email:    email,
			password: "",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().Upsert(ctx, gomock.Cond(func(x any) bool {
					u, ok := x.(*domain.User)

					return ok && u.Provider == domain.ProviderOIDC
				})).Return(nil)

				return sut, ctx
			},
			want: &domain.User{
				Email:    email,
				Name:     name,
				Provider: domain.ProviderOIDC,
			},
		},
		{
			name:     "validation error",
			email:    "invalid-email",
			password: password,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)

				return sut, ctx
			},
			wantErr: "validate user",
		},
		{
			name:     "forbidden",
			email:    email,
			password: password,
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
			name:     "unauthorized",
			email:    email,
			password: password,
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*user.Service, context.Context) {
				return user.New(nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name:     "upsert fails",
			email:    email,
			password: password,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().Upsert(ctx, gomock.Any()).Return(assert.AnError)

				return sut, ctx
			},
			wantErr: "upsert user",
		},
		{
			name:     "set password fails",
			email:    email,
			password: password,
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)
				m.store.EXPECT().SetPassword(ctx, email, gomock.Any(), true).Return(assert.AnError)

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

			got, err := sut.Create(ctx, tt.email, name, tt.password)

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

func TestService_Delete(t *testing.T) {
	t.Parallel()

	adminEmail := "admin@example.com"
	targetEmail := "target@example.com"

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
				m.store.EXPECT().Get(ctx, targetEmail).Return(&domain.User{Email: targetEmail}, nil)
				m.enforcer.EXPECT().GetGroupingPolicy().Return([][]string{
					{adminEmail, auth.RoleAdmin, auth.ObjectAll},
				})
				m.store.EXPECT().Delete(ctx, targetEmail).Return(nil)
				m.enforcer.EXPECT().DeleteUser(targetEmail).Return(nil)

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
			name: "self-deletion",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: targetEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(targetEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)

				return sut, ctx
			},
			wantErr: "cannot delete your own account",
		},
		{
			name: "user not found",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().Get(ctx, targetEmail).Return(nil, domain.ErrNotFound)

				return sut, ctx
			},
			wantErr: "get user",
		},
		{
			name: "last admin guard",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().Get(ctx, targetEmail).Return(&domain.User{Email: targetEmail}, nil)
				m.enforcer.EXPECT().GetGroupingPolicy().Return([][]string{
					{targetEmail, auth.RoleAdmin, auth.ObjectAll},
				})

				return sut, ctx
			},
			wantErr: "cannot delete the last admin",
		},
		{
			name: "delete store fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().Get(ctx, targetEmail).Return(&domain.User{Email: targetEmail}, nil)
				m.enforcer.EXPECT().GetGroupingPolicy().Return([][]string{
					{adminEmail, auth.RoleAdmin, auth.ObjectAll},
				})
				m.store.EXPECT().Delete(ctx, targetEmail).Return(assert.AnError)

				return sut, ctx
			},
			wantErr: "delete user",
		},
		{
			name: "delete enforcer fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionWrite).
					Return(true, nil)
				m.store.EXPECT().Get(ctx, targetEmail).Return(&domain.User{Email: targetEmail}, nil)
				m.enforcer.EXPECT().GetGroupingPolicy().Return([][]string{
					{adminEmail, auth.RoleAdmin, auth.ObjectAll},
				})
				m.store.EXPECT().Delete(ctx, targetEmail).Return(nil)
				m.enforcer.EXPECT().DeleteUser(targetEmail).Return(assert.AnError)

				return sut, ctx
			},
			wantErr: "delete casbin user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Delete(ctx, targetEmail)

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

func TestService_List(t *testing.T) {
	t.Parallel()

	adminEmail := "admin@example.com"
	users := []*domain.User{{Email: "a@example.com"}, {Email: "b@example.com"}}

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*user.Service, context.Context)
		errIs    error
		wantErr  string
		want     []*domain.User
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionRead).
					Return(true, nil)
				m.store.EXPECT().List(ctx).Return(users, nil)

				return sut, ctx
			},
			want: users,
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
					Enforce("user@example.com", auth.ObjectAll, auth.ObjectUser, auth.ActionRead).
					Return(false, nil)

				return sut, ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "store fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionRead).
					Return(true, nil)
				m.store.EXPECT().List(ctx).Return(nil, assert.AnError)

				return sut, ctx
			},
			wantErr: "list users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.List(ctx)

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

func TestService_Get(t *testing.T) {
	t.Parallel()

	adminEmail := "admin@example.com"
	targetEmail := "user@example.com"
	u := &domain.User{Email: targetEmail}

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*user.Service, context.Context)
		errIs    error
		wantErr  string
		want     *domain.User
	}{
		{
			name: "success",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionRead).
					Return(true, nil)
				m.store.EXPECT().Get(ctx, targetEmail).Return(u, nil)

				return sut, ctx
			},
			want: u,
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
					Enforce("user@example.com", auth.ObjectAll, auth.ObjectUser, auth.ActionRead).
					Return(false, nil)

				return sut, ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "store fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*user.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: adminEmail})
				sut, m := setupService(ctrl)

				m.enforcer.EXPECT().
					Enforce(adminEmail, auth.ObjectAll, auth.ObjectUser, auth.ActionRead).
					Return(true, nil)
				m.store.EXPECT().Get(ctx, targetEmail).Return(nil, assert.AnError)

				return sut, ctx
			},
			wantErr: "get user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Get(ctx, targetEmail)

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
