package auth

//go:generate mockgen -destination=mocks/mock_access.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth policyEnforcer

import (
	"context"
	"fmt"

	authpkg "github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type policyEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
	AddRoleForUser(user, role, domain string) error
	RemoveRoleForUser(user, role, domain string) error
	GetGroupingPolicy() [][]string
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
}

// NewAssignRoleUseCase returns a new AssignRoleUseCase.
func NewAssignRoleUseCase(enforcer policyEnforcer) *AssignRoleUseCase {
	return &AssignRoleUseCase{enforcer: enforcer}
}

// Execute assigns the role. AutoSave on the enforcer's adapter handles persistence.
func (uc *AssignRoleUseCase) Execute(ctx context.Context, subject, dom, role string) error {
	claims, ok := authpkg.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, authpkg.ObjectAll, authpkg.ObjectPolicy, authpkg.ActionWrite)
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := uc.enforcer.AddRoleForUser(subject, role, dom); err != nil {
		return fmt.Errorf("add role for user: %w", err)
	}

	return nil
}

// RevokeRoleUseCase revokes a role from a subject within a domain.
type RevokeRoleUseCase struct {
	enforcer policyEnforcer
}

// NewRevokeRoleUseCase returns a new RevokeRoleUseCase.
func NewRevokeRoleUseCase(enforcer policyEnforcer) *RevokeRoleUseCase {
	return &RevokeRoleUseCase{enforcer: enforcer}
}

// Execute revokes the role. AutoSave on the enforcer's adapter handles persistence.
func (uc *RevokeRoleUseCase) Execute(ctx context.Context, subject, dom, role string) error {
	claims, ok := authpkg.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, authpkg.ObjectAll, authpkg.ObjectPolicy, authpkg.ActionWrite)
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := uc.enforcer.RemoveRoleForUser(subject, role, dom); err != nil {
		return fmt.Errorf("remove role for user: %w", err)
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
func (uc *ListPoliciesUseCase) Execute(ctx context.Context) ([]PolicyRule, error) {
	claims, ok := authpkg.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, authpkg.ObjectAll, authpkg.ObjectPolicy, authpkg.ActionRead)
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

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
