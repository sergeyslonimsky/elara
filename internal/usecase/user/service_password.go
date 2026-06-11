package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// ResetPassword sets a new password for another user.
//
// Authorization: User:Write * (global) OR derived via authz.Scope (target
// is a member of any group the actor can write to). Plus anti-escalation:
// caller must hold every permission target currently has, because
// rewriting the password enables impersonation.
//
// Sessions: every active session of the target user is revoked inside the
// same PAP.Write transaction. The target is force-logged-out across all
// devices and must re-authenticate with the new password. Revoke runs
// BEFORE SetPassword so the credential and authority mutations land
// atomically — if SetPassword fails afterward, the revoke is rolled back
// with the rest of the tx.
//
// All checks run inside PAP.Write so they observe the same snapshot as
// the password mutation: no TOCTOU window between authorize and apply.
func (s *Service) ResetPassword(
	ctx context.Context,
	actor domain.AuthInfo,
	userID uuid.UUID,
	newPassword string,
) error {
	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	err = s.pap.Write(ctx, func(ctx context.Context, _ *authz.PAPTx) error {
		user, err := s.store.GetByID(ctx, userID)
		if err != nil {
			if errors.Is(err, storage.ErrResourceNotFound) {
				return fmt.Errorf("get user: %w", domain.ErrNotFound)
			}

			return fmt.Errorf("get user: %w", err)
		}
		if err := s.authorizeUserWrite(ctx, actor, user.ID.String()); err != nil {
			return err
		}
		if err := s.sessions.RevokeAllForUser(
			ctx,
			user.ID.String(),
			actor.UserID,
			"password reset by admin",
		); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		if err := s.store.SetPassword(ctx, user.ID, newHash, true); err != nil {
			return fmt.Errorf("set password: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("reset password transaction: %w", err)
	}

	return nil
}
