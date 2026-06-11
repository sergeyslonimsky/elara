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

// ChangePassword verifies the current password (unless forced change is
// required), invalidates every existing session of the user, hashes and
// stores the new password, and mints a fresh session that the handler
// returns as the new cookie.
//
// The revoke-all step is mandatory and runs inside the same WithTx as the
// password mutation: anyone holding a stale session ID (the current cookie
// included, plus any leaked IDs from XSS / logs / other devices) is
// authoritatively logged out atomically with the credential change. A new
// session is then created so the user remains "logged in" from their own
// browser via the freshly issued cookie.
//
// Ordering inside the tx is deliberate: revoke BEFORE SetPassword. The
// principle is "invalidate authority before changing the secret". If
// SetPassword later fails, tx rollback restores both the password and the
// session states atomically.
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
		user, err := s.users.GetByIdentity(ctx, string(domain.ProviderBasic), ctxUser.Email)
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

		if err := s.sessions.RevokeAllForUser(
			ctx,
			user.ID.String(),
			user.ID.String(),
			"password changed",
		); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}

		if err := s.pass.SetPassword(ctx, user.ID, newHash, false); err != nil {
			return fmt.Errorf("set password: %w", err)
		}

		newSess, err := s.sessions.Create(ctx, sessions.CreateParams{
			UserID:     user.ID.String(),
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
