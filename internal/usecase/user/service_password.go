package user

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

// ResetPassword sets a new password for another user.
//
// Authorization: User:Write * (global) OR derived via authz.Scope (target
// is a member of any group the actor can write to). Plus anti-escalation:
// caller must hold every permission target currently has, because
// rewriting the password enables impersonation.
//
// All checks run inside PAP.Write so they observe the same snapshot as
// the password mutation: no TOCTOU window between authorize and apply.
func (s *Service) ResetPassword(
	ctx context.Context,
	actor domain.AuthInfo,
	targetEmail, newPassword string,
) error {
	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	err = s.pap.Write(ctx, func(ctx context.Context, _ *authz.PAPTx) error {
		if _, err := s.store.Get(ctx, targetEmail); err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if err := s.authorizeUserWrite(ctx, actor, targetEmail); err != nil {
			return err
		}
		if err := s.store.SetPassword(ctx, targetEmail, newHash, true); err != nil {
			return fmt.Errorf("set password: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("reset password transaction: %w", err)
	}

	return nil
}
