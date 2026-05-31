package authz_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	authz_mock "github.com/sergeyslonimsky/elara/internal/service/authz/mocks"
)

func TestAuthz_Require(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		object   domain.Object
		action   domain.Action
		domain   string
		mockFunc func(context.Context, *gomock.Controller) (*authz.Authz, context.Context)
		wantErr  string
		errCode  connect.Code
	}{
		{
			name:   "authorized",
			object: domain.ObjectNamespace,
			action: domain.ActionRead,
			domain: domain.NamespaceResource("dom1"),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authz.Authz, context.Context) {
				ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "user@example.com"})
				pdp := authz_mock.NewMockpdp(ctrl)
				pdp.EXPECT().Has("user@example.com", domain.Permission{
					Object: domain.ObjectNamespace,
					Action: domain.ActionRead,
					Domain: domain.NamespaceResource("dom1"),
				}).Return(true)

				return authz.NewAuthz(pdp), ctx
			},
		},
		{
			name:   "unauthenticated",
			object: domain.ObjectNamespace,
			action: domain.ActionRead,
			domain: domain.NamespaceResource("dom1"),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authz.Authz, context.Context) {
				return authz.NewAuthz(nil), ctx
			},
			wantErr: "unauthenticated",
			errCode: connect.CodeUnauthenticated,
		},
		{
			name:   "unauthorized",
			object: domain.ObjectNamespace,
			action: domain.ActionRead,
			domain: domain.NamespaceResource("dom1"),
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authz.Authz, context.Context) {
				ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "user@example.com"})
				pdp := authz_mock.NewMockpdp(ctrl)
				pdp.EXPECT().Has("user@example.com", domain.Permission{
					Object: domain.ObjectNamespace,
					Action: domain.ActionRead,
					Domain: domain.NamespaceResource("dom1"),
				}).Return(false)

				return authz.NewAuthz(pdp), ctx
			},
			wantErr: "permission_denied",
			errCode: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.Require(ctx, tt.object, tt.action, tt.domain)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				var connErr *connect.Error
				require.ErrorAs(t, err, &connErr)
				assert.Equal(t, tt.errCode, connErr.Code())

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestAuthz_RequireAuthenticated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*authz.Authz, context.Context)
		wantErr  string
		errCode  connect.Code
	}{
		{
			name: "authenticated",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authz.Authz, context.Context) {
				ctx = auth2.WithClaims(ctx, &auth2.Claims{Email: "user@example.com"})

				return authz.NewAuthz(nil), ctx
			},
		},
		{
			name: "unauthenticated",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*authz.Authz, context.Context) {
				return authz.NewAuthz(nil), ctx
			},
			wantErr: "unauthenticated",
			errCode: connect.CodeUnauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut, ctx := tt.mockFunc(t.Context(), ctrl)

			err := sut.RequireAuthenticated(ctx)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				var connErr *connect.Error
				require.ErrorAs(t, err, &connErr)
				assert.Equal(t, tt.errCode, connErr.Code())

				return
			}
			require.NoError(t, err)
		})
	}
}
