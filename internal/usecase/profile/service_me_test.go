package profile_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/profile"
)

func TestService_Me(t *testing.T) {
	t.Parallel()

	const (
		email = "user@example.com"
		name  = "Test User"
	)

	tests := []struct {
		name     string
		mockFunc func(context.Context, *gomock.Controller) (*profile.Service, context.Context)
		errIs    error
		wantErr  string
		want     *profile.MeResult
	}{
		{
			name: "unauthorized - no user in context",
			mockFunc: func(ctx context.Context, _ *gomock.Controller) (*profile.Service, context.Context) {
				return profile.New(nil, nil, nil, nil, nil), ctx
			},
			errIs: domain.ErrUnauthorized,
		},
		{
			name: "pdp error is wrapped",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = authctx.WithSession(
					ctx,
					&domain.Session{UserID: testUserID},
					&domain.User{ID: uuid.MustParse(testUserID), Email: email, DisplayName: name},
				)
				svc, m := setupService(ctrl)
				m.pdp.EXPECT().
					ListPermissions(testUserID).
					Return(nil, errors.New("casbin boom"))

				return svc, ctx
			},
			wantErr: "me: casbin boom",
		},
		{
			name: "happy path - permissions passthrough",
			mockFunc: func(ctx context.Context, ctrl *gomock.Controller) (*profile.Service, context.Context) {
				ctx = authctx.WithSession(
					ctx,
					&domain.Session{UserID: testUserID},
					&domain.User{ID: uuid.MustParse(testUserID), Email: email, DisplayName: name},
				)
				svc, m := setupService(ctrl)
				m.pdp.EXPECT().
					ListPermissions(testUserID).
					Return([]domain.Permission{
						{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "ns1"},
						{Object: domain.ObjectUser, Action: domain.ActionRead, Domain: "ns2"},
					}, nil)

				return svc, ctx
			},
			want: &profile.MeResult{
				Email: email,
				Name:  name,
				Permissions: []domain.Permission{
					{Object: domain.ObjectNamespace, Action: domain.ActionRead, Domain: "ns1"},
					{Object: domain.ObjectUser, Action: domain.ActionRead, Domain: "ns2"},
				},
			},
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
