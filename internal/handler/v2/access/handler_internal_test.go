package access

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	access_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/access/mocks"
	accessv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/access/v1"
	policyuc "github.com/sergeyslonimsky/elara/internal/usecase/policy"
)

func TestAccessHandler_AssignRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mock    func(az *access_mock.Mockauthz, uc *access_mock.MockaccessUsecase)
		wantErr bool
	}{
		{
			name: "assigns role when authorized",
			mock: func(az *access_mock.Mockauthz, uc *access_mock.MockaccessUsecase) {
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectPolicy, domain.ActionWrite, domain.DomainAll).
					Return(nil)
				uc.EXPECT().
					AssignRole(gomock.Any(), "group:dev", "ns1", domain.RoleWriter).
					Return(nil)
			},
		},
		{
			name: "forbidden returns error before usecase",
			mock: func(az *access_mock.Mockauthz, _ *access_mock.MockaccessUsecase) {
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectPolicy, domain.ActionWrite, domain.DomainAll).
					Return(domain.ErrForbidden)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			az := access_mock.NewMockauthz(ctrl)
			uc := access_mock.NewMockaccessUsecase(ctrl)
			tc.mock(az, uc)

			h := NewAccessHandler(az, uc)

			_, err := h.AssignRole(t.Context(), connect.NewRequest(&accessv1.AssignRoleRequest{
				Subject: "group:dev",
				Domain:  "ns1",
				Role:    "writer",
			}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestAccessHandler_RevokeRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mock    func(az *access_mock.Mockauthz, uc *access_mock.MockaccessUsecase)
		wantErr bool
	}{
		{
			name: "revokes role when authorized",
			mock: func(az *access_mock.Mockauthz, uc *access_mock.MockaccessUsecase) {
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectPolicy, domain.ActionWrite, domain.DomainAll).
					Return(nil)
				uc.EXPECT().
					RevokeRole(gomock.Any(), "group:dev", "ns1", domain.RoleWriter).
					Return(nil)
			},
		},
		{
			name: "forbidden returns error before usecase",
			mock: func(az *access_mock.Mockauthz, _ *access_mock.MockaccessUsecase) {
				az.EXPECT().
					Require(gomock.Any(), domain.ObjectPolicy, domain.ActionWrite, domain.DomainAll).
					Return(domain.ErrForbidden)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			az := access_mock.NewMockauthz(ctrl)
			uc := access_mock.NewMockaccessUsecase(ctrl)
			tc.mock(az, uc)

			h := NewAccessHandler(az, uc)

			_, err := h.RevokeRole(t.Context(), connect.NewRequest(&accessv1.RevokeRoleRequest{
				Subject: "group:dev",
				Domain:  "ns1",
				Role:    "writer",
			}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestAccessHandler_ListPolicies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mock    func(uc *access_mock.MockaccessUsecase)
		wantLen int
		wantErr bool
	}{
		{
			name: "returns rules without authz gate",
			mock: func(uc *access_mock.MockaccessUsecase) {
				uc.EXPECT().
					List(gomock.Any()).
					Return([]policyuc.Rule{
						{Subject: "dev", Domain: "ns1", Role: "writer"},
						{Subject: "ops", Domain: "*", Role: "admin"},
					}, nil)
			},
			wantLen: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			az := access_mock.NewMockauthz(ctrl)
			uc := access_mock.NewMockaccessUsecase(ctrl)
			tc.mock(uc)

			h := NewAccessHandler(az, uc)

			resp, err := h.ListPolicies(t.Context(), connect.NewRequest(&accessv1.ListPoliciesRequest{}))

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetRules(), tc.wantLen)
		})
	}
}
