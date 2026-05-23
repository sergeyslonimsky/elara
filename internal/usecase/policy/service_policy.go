package policy

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// Rule is a usecase-level value type representing a role assignment.
// In a groups-only RBAC, the subject is always a group identified by name.
type Rule struct {
	Subject string
	Domain  string
	Role    string
}

// AssignRole grants the given role to the named group within a domain. Elara
// only supports role assignments to groups; if you need a single user to have
// a role, put them in a group and grant the role to that group.
func (s *Service) AssignRole(ctx context.Context, subject, dom, role string) error {
	if _, err := s.groups.FindByName(ctx, subject); err != nil {
		return fmt.Errorf("find group by name: %w", err)
	}

	err := s.enforcer.WriteTx(ctx, s.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		if err := txe.AddRoleForUser(casbin.GroupSubject(subject), role, dom); err != nil {
			return fmt.Errorf("add role for group: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("write tx: %w", err)
	}

	return nil
}

// RevokeRole removes a role assignment from a group within a domain.
func (s *Service) RevokeRole(ctx context.Context, subject, dom, role string) error {
	if _, err := s.groups.FindByName(ctx, subject); err != nil {
		return fmt.Errorf("find group by name: %w", err)
	}

	err := s.enforcer.WriteTx(ctx, s.txm, func(_ storage.Tx, txe *casbin.TxEnforcer) error {
		if err := txe.RemoveRoleForUser(casbin.GroupSubject(subject), role, dom); err != nil {
			return fmt.Errorf("remove role for group: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("write tx: %w", err)
	}

	return nil
}

// List returns all group->role assignments.
func (s *Service) List(_ context.Context) ([]Rule, error) {
	rules := s.enforcer.GetGroupingPolicy()

	const gRuleLen = 3

	result := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if len(rule) < gRuleLen {
			continue
		}

		if !casbin.IsGroupSubject(rule[0]) {
			continue
		}

		result = append(result, Rule{
			Subject: casbin.GroupNameFromSubject(rule[0]),
			Role:    rule[1],
			Domain:  rule[2],
		})
	}

	return result, nil
}
