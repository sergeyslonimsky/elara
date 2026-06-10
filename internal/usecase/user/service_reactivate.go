package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// ReactivateResult bundles the updated user state after reactivation.
type ReactivateResult struct {
	User *domain.User
}

// Reactivate marks a deactivated user as active.
//
// Authorization: User:Write * (global) OR derived (target ∈ any group with
// Group:Write). Plus anti-escalation: caller must hold every permission
// target currently has.
//
// Sessions are NOT restored — they were revoked at deactivate time and stay
// gone permanently (EL-50 §6.3). The user logs in afresh.
func (s *Service) Reactivate(
	ctx context.Context,
	actor domain.AuthInfo,
	userID uuid.UUID,
) (*ReactivateResult, error) {
	updated, err := s.transitionStatus(
		ctx,
		actor,
		userID,
		"cannot reactivate your own account",
		nil,
		s.users.Reactivate,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("reactivate user transaction: %w", err)
	}

	return &ReactivateResult{User: updated}, nil
}
