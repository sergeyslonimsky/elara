package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// DeactivateResult bundles the updated user state after deactivation.
type DeactivateResult struct {
	User *domain.User
}

// Deactivate marks a user as inactive and revokes all their active sessions.
//
// Both operations are performed atomically within a single transaction
// (EL-51 headline invariant).
//
// Authorization: User:Write * (global) OR derived (target ∈ any group with
// Group:Write). Plus anti-escalation: caller must hold every permission
// target currently has.
func (s *Service) Deactivate(
	ctx context.Context,
	actor domain.AuthInfo,
	userID uuid.UUID,
) (*DeactivateResult, error) {
	updated, err := s.transitionStatus(
		ctx,
		actor,
		userID,
		"cannot deactivate your own account",
		s.validateLastAdmin,
		s.users.Deactivate,
		func(ctx context.Context, user *domain.User) error {
			if err := s.sessions.RevokeAllForUser(
				ctx,
				user.ID.String(),
				actor.UserID,
				"account deactivated",
			); err != nil {
				return fmt.Errorf("revoke sessions: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("deactivate user transaction: %w", err)
	}

	return &DeactivateResult{User: updated}, nil
}
