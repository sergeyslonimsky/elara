package auth

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	internalauth "github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
)

// LoginParams carries the parameters for BasicLogin.
type LoginParams struct {
	Email     string
	Password  string
	IP        string
	UserAgent string
}

// BasicLogin verifies the user's credentials and returns a new session.
//
// User lookup, status check, and password verification run outside the tx
// — they are read-only fast-fails. Only the session create is wrapped in
// WithTx, because that is the only step with persistent state.
func (s *Service) BasicLogin(
	ctx context.Context,
	params LoginParams,
) (*domain.User, *domain.Session, error) {
	normalizedEmail, err := domain.NormalizeEmail(params.Email)
	if err != nil {
		return nil, nil, domain.ErrUnauthorized
	}

	user, err := s.users.GetByIdentity(ctx, string(domain.ProviderBasic), normalizedEmail)
	if err != nil {
		return nil, nil, domain.ErrUnauthorized
	}

	if user.Status != domain.UserStatusActive {
		return nil, nil, domain.ErrUserDeactivated
	}

	if err := internalauth.VerifyPassword(user.PasswordHash, params.Password); err != nil {
		return nil, nil, domain.ErrUnauthorized
	}

	var sess *domain.Session

	err = s.txm.WithTx(ctx, func(ctx context.Context) error {
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
		return nil, nil, fmt.Errorf("basic login tx: %w", err)
	}

	return user, sess, nil
}
