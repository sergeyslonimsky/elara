package access

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	access_mock "github.com/sergeyslonimsky/elara/internal/handler/v2/access/mocks"
	accessv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/access/v1"
	internalauth "github.com/sergeyslonimsky/elara/internal/service/auth"
	policyuc "github.com/sergeyslonimsky/elara/internal/usecase/policy"
)

func testCtx() context.Context {
	return internalauth.WithClaims(context.Background(), &internalauth.Claims{Email: "test@example.com"})
}

func TestAccessHandler_AssignRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject string
		domain  string
		role    string
		ucErr   error
		wantErr bool
	}{
		{
			name:    "assigns role successfully",
			subject: "user@example.com",
			domain:  "*",
			role:    "role:admin",
		},
		{
			name:    "usecase error propagated",
			subject: "user@example.com",
			domain:  "*",
			role:    "role:admin",
			ucErr:   errors.New("enforcer error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := access_mock.NewMockaccessUsecase(ctrl)
			uc.EXPECT().
				AssignRole(gomock.Any(), tc.subject, tc.domain, tc.role).
				Return(tc.ucErr)

			h := NewAccessHandler(uc)

			_, err := h.AssignRole(t.Context(), connect.NewRequest(&accessv1.AssignRoleRequest{
				Subject: tc.subject,
				Domain:  tc.domain,
				Role:    tc.role,
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
		subject string
		domain  string
		role    string
		ucErr   error
		wantErr bool
	}{
		{
			name:    "revokes role successfully",
			subject: "user@example.com",
			domain:  "*",
			role:    "role:admin",
		},
		{
			name:    "usecase error propagated",
			subject: "user@example.com",
			domain:  "*",
			role:    "role:admin",
			ucErr:   errors.New("enforcer error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := access_mock.NewMockaccessUsecase(ctrl)
			uc.EXPECT().
				RevokeRole(gomock.Any(), tc.subject, tc.domain, tc.role).
				Return(tc.ucErr)

			h := NewAccessHandler(uc)

			_, err := h.RevokeRole(t.Context(), connect.NewRequest(&accessv1.RevokeRoleRequest{
				Subject: tc.subject,
				Domain:  tc.domain,
				Role:    tc.role,
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
		rules   []policyuc.PolicyRule
		wantLen int
	}{
		{
			name: "returns all policies",
			rules: []policyuc.PolicyRule{
				{Subject: "user@example.com", Role: "admin", Domain: "*"},
				{Subject: "bob@example.com", Role: "reader", Domain: "ns1"},
			},
			wantLen: 2,
		},
		{
			name:    "returns empty list",
			rules:   []policyuc.PolicyRule{},
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := access_mock.NewMockaccessUsecase(ctrl)
			uc.EXPECT().List(gomock.Any()).Return(tc.rules, nil)

			h := NewAccessHandler(uc)

			resp, err := h.ListPolicies(t.Context(), connect.NewRequest(&accessv1.ListPoliciesRequest{}))
			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetRules(), tc.wantLen)
		})
	}
}
