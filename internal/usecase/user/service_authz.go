package user

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// authorizeUserWrite is the two-step authorization shared by ResetPassword
// and Delete:
//
//  1. Scope: actor must hold User:Write * OR have Group:Write on at least
//     one group target belongs to (delegated to authz.Scope).
//  2. Anti-escalation: actor must hold every permission target effectively
//     has — otherwise a password reset would enable impersonation that
//     elevates the actor beyond their own boundary.
func (s *Service) authorizeUserWrite(
	ctx context.Context,
	actor domain.AuthInfo,
	targetEmail string,
) error {
	if err := s.scope.RequireWriteUser(ctx, actor.Email, targetEmail); err != nil {
		return fmt.Errorf("require write user: %w", err)
	}

	return s.checkAntiEscalation(actor, targetEmail)
}

// checkAntiEscalation asserts the actor's effective permissions are a
// superset of target's. Used wherever a write operation could indirectly
// transfer the target's capabilities to the actor (impersonation,
// destructive admin).
func (s *Service) checkAntiEscalation(actor domain.AuthInfo, targetEmail string) error {
	targetPerms, err := s.pdp.ListPermissions(targetEmail)
	if err != nil {
		return fmt.Errorf("list target permissions: %w", err)
	}
	for _, p := range targetPerms {
		if !s.pdp.Has(actor.Email, p) {
			return domain.ErrPermissionEscalation
		}
	}

	return nil
}
