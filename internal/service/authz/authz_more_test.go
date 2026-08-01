package authz_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	authz_mock "github.com/sergeyslonimsky/elara/internal/service/authz/mocks"
)

func TestAuthz_RequireUser(t *testing.T) {
	t.Parallel()

	user := domain.AuthInfo{UserID: testUserID}

	tests := []struct {
		name     string
		mockFunc func(*gomock.Controller) *authz.Authz
		wantErr  string
		errCode  connect.Code
	}{
		{
			name: "authorized",
			mockFunc: func(ctrl *gomock.Controller) *authz.Authz {
				pdp := authz_mock.NewMockpdp(ctrl)
				pdp.EXPECT().Has(testUserID, domain.Permission{
					Object: domain.ObjectNamespace,
					Action: domain.ActionRead,
					Domain: "dom1",
				}).Return(true)

				return authz.NewAuthz(pdp)
			},
		},
		{
			name: "unauthorized",
			mockFunc: func(ctrl *gomock.Controller) *authz.Authz {
				pdp := authz_mock.NewMockpdp(ctrl)
				pdp.EXPECT().Has(testUserID, domain.Permission{
					Object: domain.ObjectNamespace,
					Action: domain.ActionRead,
					Domain: "dom1",
				}).Return(false)

				return authz.NewAuthz(pdp)
			},
			wantErr: "permission_denied",
			errCode: connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			err := sut.RequireUser(user, domain.ObjectNamespace, domain.ActionRead, "dom1")

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

func TestAuthz_RequireNamespace(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	pdp := authz_mock.NewMockpdp(ctrl)
	pdp.EXPECT().Has(testUserID, domain.Permission{
		Object: domain.ObjectNamespace,
		Action: domain.ActionWrite,
		Domain: domain.NamespaceResource("prod"),
	}).Return(true)

	sut := authz.NewAuthz(pdp)
	ctx := withTestUser(t.Context(), "user@example.com")

	require.NoError(t, sut.RequireNamespace(ctx, domain.ActionWrite, "prod"))
}

func TestAuthz_RequireGroup(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	pdp := authz_mock.NewMockpdp(ctrl)
	pdp.EXPECT().Has(testUserID, domain.Permission{
		Object: domain.ObjectGroup,
		Action: domain.ActionWrite,
		Domain: domain.GroupResource("devs"),
	}).Return(false)

	sut := authz.NewAuthz(pdp)
	ctx := withTestUser(t.Context(), "user@example.com")

	err := sut.RequireGroup(ctx, domain.ActionWrite, "devs")
	require.ErrorContains(t, err, "permission_denied")
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}
