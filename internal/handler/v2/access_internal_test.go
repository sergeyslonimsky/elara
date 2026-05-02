package v2

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	authv1 "github.com/sergeyslonimsky/elara/internal/proto/elara/auth/v1"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestAccessHandler_AssignRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		subject       string
		domain        string
		role          string
		addErr        error
		savePolicyErr error
		wantErr       bool
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
			loader := auth_mock.NewMockaccessPolicyLoader(ctrl)

			enforcer.EXPECT().AddRoleForUser(tc.subject, tc.role, tc.domain).Return(tc.addErr)
			if tc.addErr == nil {
				enforcer.EXPECT().SavePolicy(gomock.Any()).Return(tc.savePolicyErr)
			}

			h := NewAccessHandler(
				authuc.NewAssignRoleUseCase(enforcer, loader),
				nil, nil,
			)

			_, err := h.AssignRole(context.Background(), connect.NewRequest(&authv1.AssignRoleRequest{
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
			loader := auth_mock.NewMockaccessPolicyLoader(ctrl)

			enforcer.EXPECT().RemoveRoleForUser(tc.subject, tc.role, tc.domain).Return(tc.removeErr)
			if tc.removeErr == nil {
				enforcer.EXPECT().SavePolicy(gomock.Any()).Return(nil)
			}

			h := NewAccessHandler(
				nil,
				authuc.NewRevokeRoleUseCase(enforcer, loader),
				nil,
			)

			_, err := h.RevokeRole(context.Background(), connect.NewRequest(&authv1.RevokeRoleRequest{
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
			rules:   [][]string{{"user@example.com", "role:admin", "*"}, {"bob@example.com", "role:viewer", "ns1"}},
			wantLen: 2,
		},
		{
			name:    "returns empty list",
			rules:   [][]string{},
			wantLen: 0,
		},
		{
			name:    "skips malformed rules",
			rules:   [][]string{{"only-two", "fields"}, {"user@example.com", "role:admin", "*"}},
			wantLen: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
			enforcer.EXPECT().GetGroupingPolicy().Return(tc.rules)

			h := NewAccessHandler(
				nil, nil,
				authuc.NewListPoliciesUseCase(enforcer),
			)

			resp, err := h.ListPolicies(context.Background(), connect.NewRequest(&authv1.ListPoliciesRequest{}))
			require.NoError(t, err)
			assert.Len(t, resp.Msg.GetRules(), tc.wantLen)
		})
	}
}
