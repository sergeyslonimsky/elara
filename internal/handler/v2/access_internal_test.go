package v2

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestAccessHandler_AssignRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject string
		domain  string
		role    string
		addErr  error
		wantErr bool
	}{
		{
			name:    "assigns role successfully",
			subject: "user@example.com",
			domain:  "*",
			role:    "role:admin",
		},
		{
			name:    "enforcer error propagated",
			subject: "user@example.com",
			domain:  "*",
			role:    "role:admin",
			addErr:  errors.New("enforcer error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
			groups := auth_mock.NewMockgroupByNameFinder(ctrl)

			enforcer.EXPECT().Enforce(gomock.Any(), "*", "policy", "write").Return(true, nil)
			enforcer.EXPECT().AddRoleForUser(tc.subject, tc.role, tc.domain).Return(tc.addErr)
			if tc.addErr == nil {
				groups.EXPECT().FindByName(gomock.Any(), tc.subject).
					Return(nil, domain.NewNotFoundError("group", tc.subject))
			}

			h := NewAccessHandler(
				authuc.NewAssignRoleUseCase(enforcer, groups),
				nil, nil,
			)

			_, err := h.AssignRole(clientsHandlerTestCtx(), connect.NewRequest(&authv1.AssignRoleRequest{
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
		name      string
		subject   string
		domain    string
		role      string
		removeErr error
		wantErr   bool
	}{
		{
			name:    "revokes role successfully",
			subject: "user@example.com",
			domain:  "*",
			role:    "role:admin",
		},
		{
			name:      "enforcer error propagated",
			subject:   "user@example.com",
			domain:    "*",
			role:      "role:admin",
			removeErr: errors.New("enforcer error"),
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
			groups := auth_mock.NewMockgroupByNameFinder(ctrl)

			enforcer.EXPECT().Enforce(gomock.Any(), "*", "policy", "write").Return(true, nil)
			enforcer.EXPECT().RemoveRoleForUser(tc.subject, tc.role, tc.domain).Return(tc.removeErr)
			if tc.removeErr == nil {
				groups.EXPECT().FindByName(gomock.Any(), tc.subject).
					Return(nil, domain.NewNotFoundError("group", tc.subject))
			}

			h := NewAccessHandler(
				nil,
				authuc.NewRevokeRoleUseCase(enforcer, groups),
				nil,
			)

			_, err := h.RevokeRole(clientsHandlerTestCtx(), connect.NewRequest(&authv1.RevokeRoleRequest{
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
		rules   [][]string
		wantLen int
	}{
		{
			name:    "returns all policies",
			rules:   [][]string{{"user@example.com", "admin", "*"}, {"bob@example.com", "reader", "ns1"}},
			wantLen: 2,
		},
		{
			name:    "returns empty list",
			rules:   [][]string{},
			wantLen: 0,
		},
		{
			name:    "skips malformed rules",
			rules:   [][]string{{"only-two", "fields"}, {"user@example.com", "admin", "*"}},
			wantLen: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
			enforcer.EXPECT().Enforce(gomock.Any(), "*", "policy", "read").Return(true, nil)
			enforcer.EXPECT().GetGroupingPolicy().Return(tc.rules)

			h := NewAccessHandler(
				nil, nil,
				authuc.NewListPoliciesUseCase(enforcer),
			)

			resp, err := h.ListPolicies(clientsHandlerTestCtx(), connect.NewRequest(&authv1.ListPoliciesRequest{}))
			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetRules(), tc.wantLen)
		})
	}
}
