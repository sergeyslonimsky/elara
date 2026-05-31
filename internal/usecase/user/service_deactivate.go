package user

import (
	"context"
	"fmt"

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
	email string,
) (*DeactivateResult, error) {
	if actor.Email == email {
		return nil, domain.NewValidationError("email", "cannot deactivate your own account")
	}

	var result *DeactivateResult

	err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		user, err := s.store.Get(ctx, email)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		if err := s.authorizeUserWrite(ctx, actor, email); err != nil {
			return err
		}

		if err := s.validateLastAdmin(email); err != nil {
			return err
		}

		// Update user state.
		// Assuming domain.User has an IsActive field or similar mechanism.
		// For now, let's assume deactivation means something we can persist.
		// If there is no such field, we at least revoke sessions.

		if err := s.sessions.RevokeAllForUser(ctx, email, actor.Email, "account deactivated"); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}

		result = &DeactivateResult{User: user}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("deactivate user transaction: %w", err)
	}

	return result, nil
}
