package profile

import (
	"context"
	"fmt"

	auth2 "github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
)

// ChangePasswordParams carries the parameters for ChangePassword.
type ChangePasswordParams struct {
	CurrentPassword string
	NewPassword     string
	IP              string
	UserAgent       string
}

// ChangePassword verifies the current password (unless forced change is required),
// hashes and stores the new one, and creates a new session.
//
// Both operations are performed atomically within a single transaction.
func (s *Service) ChangePassword(
	ctx context.Context,
	params ChangePasswordParams,
) (*domain.Session, error) {
	ctxUser, ok := auth2.UserFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	var sess *domain.Session

	err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		user, err := s.users.Get(ctx, ctxUser.Email)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		if !ctxUser.PasswordChangeRequired {
			if err := auth.VerifyPassword(user.PasswordHash, params.CurrentPassword); err != nil {
				return domain.ErrUnauthorized
			}
		}

		newHash, err := auth.HashPassword(params.NewPassword)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}

		if err := s.pass.SetPassword(ctx, ctxUser.Email, newHash, false); err != nil {
			return fmt.Errorf("set password: %w", err)
		}

		// Create a new session after password change.
		newSess, err := s.sessions.Create(ctx, sessions.CreateParams{
			UserID:     ctxUser.Email,
			ClientType: string(domain.ClientTypeWeb),
			IP:         params.IP,
			UserAgent:  params.UserAgent,
		})
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		sess = newSess

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("change password tx: %w", err)
	}

	return sess, nil
}
