package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	mockauth "github.com/sergeyslonimsky/elara/internal/service/auth/mocks"
)

func TestCheckAccess(t *testing.T) {
	t.Parallel()

	const (
		dom = "test-domain"
		obj = "test-object"
		act = "read"
	)

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (context.Context, auth.AccessEnforcer)
		errIs    error
		wantErr  string
	}{
		{
			name: "no claims",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (context.Context, auth.AccessEnforcer) {
				return ctx, mockauth.NewMockAccessEnforcer(ctrl)
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "enforcer error",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (context.Context, auth.AccessEnforcer) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				e := mockauth.NewMockAccessEnforcer(ctrl)
				e.EXPECT().
					Enforce("user@example.com", dom, obj, act).
					Return(false, errors.New("db error"))

				return ctx, e
			},
			wantErr: "enforce: db error",
		},
		{
			name: "forbidden",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (context.Context, auth.AccessEnforcer) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				e := mockauth.NewMockAccessEnforcer(ctrl)
				e.EXPECT().Enforce("user@example.com", dom, obj, act).Return(false, nil)

				return ctx, e
			},
			errIs: domain.ErrForbidden,
		},
		{
			name: "allowed",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (context.Context, auth.AccessEnforcer) {
				ctx = auth.WithClaims(ctx, &auth.Claims{Email: "user@example.com"})
				e := mockauth.NewMockAccessEnforcer(ctrl)
				e.EXPECT().Enforce("user@example.com", dom, obj, act).Return(true, nil)

				return ctx, e
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			ctx, e := tt.mockFunc(t.Context(), ctrl)

			err := auth.CheckAccess(ctx, e, dom, obj, act)

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
