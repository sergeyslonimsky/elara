package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

type fakePolicyEnforcer struct {
	gRules    [][]string
	addErr    error
	removeErr error
	saveErr   error
}

func (f *fakePolicyEnforcer) AddRoleForUser(_, _, _ string) error {
	return f.addErr
}

func (f *fakePolicyEnforcer) RemoveRoleForUser(_, _, _ string) error {
	return f.removeErr
}

func (f *fakePolicyEnforcer) GetGroupingPolicy() [][]string {
	return f.gRules
}

func (f *fakePolicyEnforcer) SavePolicy(_ casbin.PolicyLoader) error {
	return f.saveErr
}

type fakePolicyPersister struct {
	err error
}

func (f *fakePolicyPersister) Load(_ context.Context) ([][]string, error) {
	return nil, nil
}

func (f *fakePolicyPersister) Save(_ context.Context, _ [][]string) error {
	return f.err
}

func TestAssignRevokeRoleUseCase_Execute(t *testing.T) {
	t.Parallel()

	type ucExec interface {
		Execute(ctx context.Context, subject, domain, role string) error
	}

	tests := []struct {
		name      string
		buildUC   func(e *fakePolicyEnforcer, p *fakePolicyPersister) ucExec
		enforcer  *fakePolicyEnforcer
		persister *fakePolicyPersister
		wantErr   bool
	}{
		{
			name:      "assigns role and saves policy",
			buildUC:   func(e *fakePolicyEnforcer, p *fakePolicyPersister) ucExec { return authuc.NewAssignRoleUseCase(e, p) },
			enforcer:  &fakePolicyEnforcer{},
			persister: &fakePolicyPersister{},
		},
		{
			name:      "assign: enforcer error propagated",
			buildUC:   func(e *fakePolicyEnforcer, p *fakePolicyPersister) ucExec { return authuc.NewAssignRoleUseCase(e, p) },
			enforcer:  &fakePolicyEnforcer{addErr: errors.New("enforcer error")},
			persister: &fakePolicyPersister{},
			wantErr:   true,
		},
		{
			name:      "revokes role and saves policy",
			buildUC:   func(e *fakePolicyEnforcer, p *fakePolicyPersister) ucExec { return authuc.NewRevokeRoleUseCase(e, p) },
			enforcer:  &fakePolicyEnforcer{},
			persister: &fakePolicyPersister{},
		},
		{
			name:      "revoke: enforcer error propagated",
			buildUC:   func(e *fakePolicyEnforcer, p *fakePolicyPersister) ucExec { return authuc.NewRevokeRoleUseCase(e, p) },
			enforcer:  &fakePolicyEnforcer{removeErr: errors.New("enforcer error")},
			persister: &fakePolicyPersister{},
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			uc := tc.buildUC(tc.enforcer, tc.persister)
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

			uc := authuc.NewListPoliciesUseCase(&fakePolicyEnforcer{gRules: tc.rules})
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
