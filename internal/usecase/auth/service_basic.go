package auth

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	internalauth "github.com/sergeyslonimsky/elara/internal/service/auth"
)

// BasicLogin verifies the user's credentials, optionally bootstraps the
// configured admin into the admins group, and returns a signed session token.
func (s *Service) BasicLogin(ctx context.Context, email, password string) (string, *domain.User, error) {
	user, err := s.users.Get(ctx, email)
	if err != nil {
		return "", nil, domain.ErrUnauthorized
	}

	if err := internalauth.VerifyPassword(user.PasswordHash, password); err != nil {
		return "", nil, domain.ErrUnauthorized
	}

	if email == s.adminEmail {
		if err := s.admin.EnsureMember(ctx, email); err != nil {
			return "", nil, fmt.Errorf("bootstrap admin: %w", err)
		}
	}

	token, err := s.session.Create(user)
	if err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}

	return token, user, nil
}
