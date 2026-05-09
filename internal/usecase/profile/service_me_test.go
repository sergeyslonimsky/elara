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

func TestService_Me(t *testing.T) {
	t.Parallel()

	email := "user@example.com"
	name := "Test User"
	namespaces := []*domain.Namespace{
		{Name: "prod"},
		{Name: "stage"},
	}

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*profile.Service, context.Context)
		errIs    error
		wantErr  string
		want     *profile.MeResult
	}{
		{
			name: "success fully authorized",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: email, Name: name})
				svc, m := setupService(ctrl)

				m.ns.EXPECT().List(ctx).Return(namespaces, nil)

				// Namespace "prod"
				m.enforcer.EXPECT().Enforce(email, "prod", auth.ObjectConfig, auth.ActionRead).Return(true, nil)
				m.enforcer.EXPECT().Enforce(email, "prod", auth.ObjectConfig, auth.ActionWrite).Return(true, nil)
				m.enforcer.EXPECT().Enforce(email, "prod", auth.ObjectWebhook, auth.ActionRead).Return(true, nil)

				// Namespace "stage"
				m.enforcer.EXPECT().Enforce(email, "stage", auth.ObjectConfig, auth.ActionRead).Return(true, nil)
				m.enforcer.EXPECT().Enforce(email, "stage", auth.ObjectConfig, auth.ActionWrite).Return(false, nil)

				m.enforcer.EXPECT().Enforce(email, auth.ObjectAll, auth.ObjectUser, auth.ActionRead).Return(true, nil)
				m.enforcer.EXPECT().
					Enforce(email, auth.ObjectAll, auth.ObjectWebhook, auth.ActionWrite).
					Return(true, nil)

				return svc, ctx
			},
			want: &profile.MeResult{
				Email:   email,
				Name:    name,
				IsAdmin: true,
				Namespaces: []profile.NamespaceAccess{
					{Name: "prod", CanWrite: true},
					{Name: "stage", CanWrite: false},
				},
				CanViewWebhooks:   true,
				CanManageWebhooks: true,
			},
		},
		{
			name: "success partial access",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: email, Name: name})
				svc, m := setupService(ctrl)

				m.ns.EXPECT().List(ctx).Return(namespaces, nil)

				// Namespace "prod" - no read access
				m.enforcer.EXPECT().Enforce(email, "prod", auth.ObjectConfig, auth.ActionRead).Return(false, nil)

				// Namespace "stage" - read only
				m.enforcer.EXPECT().Enforce(email, "stage", auth.ObjectConfig, auth.ActionRead).Return(true, nil)
				m.enforcer.EXPECT().Enforce(email, "stage", auth.ObjectConfig, auth.ActionWrite).Return(false, nil)
				m.enforcer.EXPECT().Enforce(email, "stage", auth.ObjectWebhook, auth.ActionRead).Return(false, nil)

				m.enforcer.EXPECT().Enforce(email, auth.ObjectAll, auth.ObjectUser, auth.ActionRead).Return(false, nil)
				m.enforcer.EXPECT().
					Enforce(email, auth.ObjectAll, auth.ObjectWebhook, auth.ActionWrite).
					Return(false, nil)

				return svc, ctx
			},
			want: &profile.MeResult{
				Email:   email,
				Name:    name,
				IsAdmin: false,
				Namespaces: []profile.NamespaceAccess{
					{Name: "stage", CanWrite: false},
				},
				CanViewWebhooks:   false,
				CanManageWebhooks: false,
			},
		},
		{
			name: "unauthorized - no claims",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				svc := profile.New(nil, nil, nil, nil, nil)

				return svc, ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "list namespaces fails",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: email, Name: name})
				svc, m := setupService(ctrl)

				m.ns.EXPECT().List(ctx).Return(nil, errors.New("db error"))

				return svc, ctx
			},
			wantErr: "list namespaces: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			got, err := sut.Me(ctx)

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
