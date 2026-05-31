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
// Both operations are performed atomically within a single transaction.
func (s *Service) BasicLogin(
	ctx context.Context,
	params LoginParams,
) (*domain.User, *domain.Session, error) {
	user, err := s.users.Get(ctx, params.Email)
	if err != nil {
		return nil, nil, domain.ErrUnauthorized
	}

	if err := internalauth.VerifyPassword(user.PasswordHash, params.Password); err != nil {
		return nil, nil, domain.ErrUnauthorized
	}

	var sess *domain.Session

	err = s.txm.WithTx(ctx, func(ctx context.Context) error {
		newSess, err := s.sessions.Create(ctx, sessions.CreateParams{
			UserID:     user.Email,
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
