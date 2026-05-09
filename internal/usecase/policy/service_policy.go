package policy

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	authpkg "github.com/sergeyslonimsky/elara/internal/service/auth"
)

// PolicyRule is a usecase-level value type representing a role assignment.
type PolicyRule struct {
	Subject string
	Domain  string
	Role    string
}

// AssignRole assigns a role to a subject within a domain.
//
//nolint:dupl // assign and revoke share the same structure intentionally
func (s *Service) AssignRole(ctx context.Context, subject, dom, role string) error {
	claims, ok := authpkg.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, authpkg.ObjectAll, authpkg.ObjectPolicy, authpkg.ActionWrite)
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := s.enforcer.AddRoleForUser(subject, role, dom); err != nil {
		return fmt.Errorf("add role for user: %w", err)
	}

	group, err := s.groups.FindByName(ctx, subject)
	if err == nil {
		for _, member := range group.Members {
			if err := s.enforcer.AddRoleForUser(member, subject, dom); err != nil {
				return fmt.Errorf("sync group member: %w", err)
			}
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("find group by name: %w", err)
	}

	return nil
}

// RevokeRole revokes a role from a subject within a domain.
//
//nolint:dupl // assign and revoke share the same structure intentionally
func (s *Service) RevokeRole(ctx context.Context, subject, dom, role string) error {
	claims, ok := authpkg.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, authpkg.ObjectAll, authpkg.ObjectPolicy, authpkg.ActionWrite)
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := s.enforcer.RemoveRoleForUser(subject, role, dom); err != nil {
		return fmt.Errorf("remove role for user: %w", err)
	}

	group, err := s.groups.FindByName(ctx, subject)
	if err == nil {
		for _, member := range group.Members {
			if err := s.enforcer.RemoveRoleForUser(member, subject, dom); err != nil {
				return fmt.Errorf("sync revoke group member: %w", err)
			}
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("find group by name: %w", err)
	}

	return nil
}

// List returns all role assignment rules.
func (s *Service) List(ctx context.Context) ([]PolicyRule, error) {
	claims, ok := authpkg.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, authpkg.ObjectAll, authpkg.ObjectPolicy, authpkg.ActionRead)
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	rules := s.enforcer.GetGroupingPolicy()

	knownRoles := map[string]bool{
		authpkg.RoleAdmin: true, authpkg.RoleWriter: true, authpkg.RoleReader: true,
	}

	result := make([]PolicyRule, 0, len(rules))
	for _, rule := range rules {
		if len(rule) < 3 { //nolint:mnd // 3 is the number of fields in a g rule
			continue
		}

		if !knownRoles[rule[1]] {
			continue // skip membership records (rule[1] = group name)
		}

		result = append(result, PolicyRule{Subject: rule[0], Role: rule[1], Domain: rule[2]})
	}

	return result, nil
}
