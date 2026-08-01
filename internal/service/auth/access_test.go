package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/service/auth/mocks"
)

func TestCheckAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*auth_mock.MockAccessEnforcer, context.Context)
		errIs    error
		wantErr  string
	}{
		{
			name: "no claims in context returns unauthorized",
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*auth_mock.MockAccessEnforcer, context.Context) {
				return nil, ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "enforcer returns error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*auth_mock.MockAccessEnforcer, context.Context) {
				ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "alice@example.com"})
				e := auth_mock.NewMockAccessEnforcer(ctrl)
				e.EXPECT().
					Enforce("alice@example.com", "ns1", "config", "write").
					Return(false, errors.New("enforcer down"))

				return e, ctx
			},
			wantErr: "enforce: enforcer down",
		},
		{
			name: "not allowed returns forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*auth_mock.MockAccessEnforcer, context.Context) {
				ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "alice@example.com"})
				e := auth_mock.NewMockAccessEnforcer(ctrl)
				e.EXPECT().Enforce("alice@example.com", "ns1", "config", "write").Return(false, nil)

				return e, ctx
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "allowed returns nil",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*auth_mock.MockAccessEnforcer, context.Context) {
				ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "alice@example.com"})
				e := auth_mock.NewMockAccessEnforcer(ctrl)
				e.EXPECT().Enforce("alice@example.com", "ns1", "config", "write").Return(true, nil)

				return e, ctx
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer, ctx := tt.mockFunc(t.Context(), ctrl)

			var err error
			if enforcer == nil {
				err = auth.CheckAccess(ctx, nil, "ns1", "config", "write")
			} else {
				err = auth.CheckAccess(ctx, enforcer, "ns1", "config", "write")
			}

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

func TestRequireAuthenticated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ctxFunc func(context.Context) context.Context
		errIs   error
	}{
		{
			name: "authenticated",
			ctxFunc: func(ctx context.Context) context.Context {
				return auth2.WithClaims(ctx, &auth2.Claims{Email: "alice@example.com"})
			},
		},
		{
			name: "no claims returns unauthorized",
			ctxFunc: func(ctx context.Context) context.Context {
				return ctx
			},
			errIs: domain.ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := auth.RequireAuthenticated(tt.ctxFunc(t.Context()))

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			require.NoError(t, err)
		})
	}
}
