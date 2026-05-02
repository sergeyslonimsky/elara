package auth

//go:generate mockgen -destination=mocks/mock_access.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth policyEnforcer,accessPolicyLoader

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
)

type policyEnforcer interface {
	AddRoleForUser(user, role, domain string) error
	RemoveRoleForUser(user, role, domain string) error
	GetGroupingPolicy() [][]string
	SavePolicy(ctx context.Context, loader casbin.PolicyLoader) error
}

type accessPolicyLoader interface {
	Load(ctx context.Context) ([][]string, error)
	Save(ctx context.Context, rules [][]string) error
}

// PolicyRule is a usecase-level value type representing a role assignment.
type PolicyRule struct {
	Subject string
	Domain  string
	Role    string
}

// AssignRoleUseCase assigns a role to a subject within a domain.
type AssignRoleUseCase struct {
	enforcer policyEnforcer
	policy   accessPolicyLoader
}

// NewAssignRoleUseCase returns a new AssignRoleUseCase.
func NewAssignRoleUseCase(enforcer policyEnforcer, policy accessPolicyLoader) *AssignRoleUseCase {
	return &AssignRoleUseCase{enforcer: enforcer, policy: policy}
}

// Execute assigns the role and persists the updated policy.
func (uc *AssignRoleUseCase) Execute(ctx context.Context, subject, domain, role string) error {
	if err := uc.enforcer.AddRoleForUser(subject, role, domain); err != nil {
		return fmt.Errorf("add role for user: %w", err)
	}

	if err := uc.enforcer.SavePolicy(ctx, uc.policy); err != nil {
		return fmt.Errorf("save policy: %w", err)
	}

	return nil
}

// RevokeRoleUseCase revokes a role from a subject within a domain.
type RevokeRoleUseCase struct {
	enforcer policyEnforcer
	policy   accessPolicyLoader
}

// NewRevokeRoleUseCase returns a new RevokeRoleUseCase.
func NewRevokeRoleUseCase(enforcer policyEnforcer, policy accessPolicyLoader) *RevokeRoleUseCase {
	return &RevokeRoleUseCase{enforcer: enforcer, policy: policy}
}

// Execute revokes the role and persists the updated policy.
func (uc *RevokeRoleUseCase) Execute(ctx context.Context, subject, domain, role string) error {
	if err := uc.enforcer.RemoveRoleForUser(subject, role, domain); err != nil {
		return fmt.Errorf("remove role for user: %w", err)
	}

	if err := uc.enforcer.SavePolicy(ctx, uc.policy); err != nil {
		return fmt.Errorf("save policy: %w", err)
	}

	return nil
}

// ListPoliciesUseCase returns all role assignment rules.
type ListPoliciesUseCase struct {
	enforcer policyEnforcer
}

// NewListPoliciesUseCase returns a new ListPoliciesUseCase.
func NewListPoliciesUseCase(enforcer policyEnforcer) *ListPoliciesUseCase {
	return &ListPoliciesUseCase{enforcer: enforcer}
}

// Execute returns all role assignment (g) rules as PolicyRule values.
func (uc *ListPoliciesUseCase) Execute(_ context.Context) ([]PolicyRule, error) {
	rules := uc.enforcer.GetGroupingPolicy()

	result := make([]PolicyRule, 0, len(rules))
	for _, rule := range rules {
		// g rules have the form: [user, role, domain]
		if len(rule) < 3 { //nolint:mnd // 3 is the number of fields in a g rule
			continue
		}

		result = append(result, PolicyRule{
			Subject: rule[0],
			Role:    rule[1],
			Domain:  rule[2],
		})
	}

	return result, nil
}
