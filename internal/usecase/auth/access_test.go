package auth_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func TestAssignRevokeRoleUseCase_Execute(t *testing.T) {
	t.Parallel()

	type ucExec interface {
		Execute(ctx context.Context, subject, domain, role string) error
	}

	tests := []struct {
		name      string
		buildUC   func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader) ucExec
		setupMock func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader)
		wantErr   bool
	}{
		{
			name: "assigns role and saves policy",
			buildUC: func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader) ucExec {
				return authuc.NewAssignRoleUseCase(e, p)
			},
			setupMock: func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader) {
				e.EXPECT().AddRoleForUser("user@example.com", "role:admin", "*").Return(nil)
				e.EXPECT().SavePolicy(gomock.Any()).Return(nil)
			},
		},
		{
			name: "assign: enforcer add error propagated",
			buildUC: func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader) ucExec {
				return authuc.NewAssignRoleUseCase(e, p)
			},
			setupMock: func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader) {
				e.EXPECT().AddRoleForUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("enforcer error"))
			},
			wantErr: true,
		},
		{
			name: "assign: save policy error propagated",
			buildUC: func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader) ucExec {
				return authuc.NewAssignRoleUseCase(e, p)
			},
			setupMock: func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader) {
				e.EXPECT().AddRoleForUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				e.EXPECT().SavePolicy(gomock.Any()).Return(errors.New("save error"))
			},
			wantErr: true,
		},
		{
			name: "revokes role and saves policy",
			buildUC: func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader) ucExec {
				return authuc.NewRevokeRoleUseCase(e, p)
			},
			setupMock: func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader) {
				e.EXPECT().RemoveRoleForUser("user@example.com", "role:admin", "*").Return(nil)
				e.EXPECT().SavePolicy(gomock.Any()).Return(nil)
			},
		},
		{
			name: "revoke: enforcer remove error propagated",
			buildUC: func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader) ucExec {
				return authuc.NewRevokeRoleUseCase(e, p)
			},
			setupMock: func(e *auth_mock.MockpolicyEnforcer, p *auth_mock.MockaccessPolicyLoader) {
				e.EXPECT().
					RemoveRoleForUser(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("enforcer error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
			persister := auth_mock.NewMockaccessPolicyLoader(ctrl)

			tc.setupMock(enforcer, persister)

			uc := tc.buildUC(enforcer, persister)
			err := uc.Execute(t.Context(), "user@example.com", "*", "role:admin")

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestListPoliciesUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rules   [][]string
		wantLen int
	}{
		{
			name:    "converts g rules to PolicyRule slice",
			rules:   [][]string{{"user@example.com", "role:admin", "*"}, {"user2@example.com", "role:viewer", "ns1"}},
			wantLen: 2,
		},
		{
			name:    "skips malformed rules",
			rules:   [][]string{{"user@example.com", "role:admin"}, {"valid@example.com", "role:viewer", "*"}},
			wantLen: 1,
		},
		{
			name:    "empty rules returns empty slice",
			rules:   [][]string{},
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMockpolicyEnforcer(ctrl)
			enforcer.EXPECT().GetGroupingPolicy().Return(tc.rules)

			uc := authuc.NewListPoliciesUseCase(enforcer)
			got, err := uc.Execute(t.Context())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != tc.wantLen {
				t.Errorf("got %d rules, want %d", len(got), tc.wantLen)
			}
		})
	}
}
