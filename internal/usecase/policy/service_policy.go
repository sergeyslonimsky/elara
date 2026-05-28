package policy

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// Rule is a usecase-level value type representing a role assignment.
// In a groups-only RBAC, the subject is always a group identified by name.
type Rule struct {
	Subject string
	Domain  string
	Role    domain.Role
}

// AssignRole grants the given role to the named group within a domain. Elara
// only supports role assignments to groups; if you need a single user to have
// a role, put them in a group and grant the role to that group.
func (s *Service) AssignRole(ctx context.Context, subject, dom string, role domain.Role) error {
	if _, err := s.groups.FindByName(ctx, subject); err != nil {
		return fmt.Errorf("find group by name: %w", err)
	}

	if err := s.pap.Write(ctx, func(_ storage.Tx, w *authz.PAPTx) error {
		return w.AssignRoleToGroup(subject, role, dom)
	}); err != nil {
		return fmt.Errorf("assign role: %w", err)
	}

	return nil
}

// RevokeRole removes a role assignment from a group within a domain.
func (s *Service) RevokeRole(ctx context.Context, subject, dom string, role domain.Role) error {
	if _, err := s.groups.FindByName(ctx, subject); err != nil {
		return fmt.Errorf("find group by name: %w", err)
	}

	if err := s.pap.Write(ctx, func(_ storage.Tx, w *authz.PAPTx) error {
		return w.RevokeRoleFromGroup(subject, role, dom)
	}); err != nil {
		return fmt.Errorf("revoke role: %w", err)
	}

	return nil
}

// List returns all group→role assignments. Direct user grants and
// membership rules are filtered out by PAP.ListGroupRoleAssignments.
func (s *Service) List(_ context.Context) ([]Rule, error) {
	assignments := s.pap.ListGroupRoleAssignments()

	result := make([]Rule, 0, len(assignments))
	for _, a := range assignments {
		result = append(result, Rule{Subject: a.Group, Role: a.Role, Domain: a.Domain})
	}

	return result, nil
}
