package auth

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	internalauth "github.com/sergeyslonimsky/elara/internal/service/auth"
)

// BasicLogin verifies the user's credentials and returns a signed session
// token. The basic-auth admin is seeded into the superadmin group at bootstrap
// (see auth.AdminBootstrap.BootstrapBasic), so no per-login elevation is
// needed here.
func (s *Service) BasicLogin(ctx context.Context, email, password string) (string, *domain.User, error) {
	user, err := s.users.Get(ctx, email)
	if err != nil {
		return "", nil, domain.ErrUnauthorized
	}

	if err := internalauth.VerifyPassword(user.PasswordHash, password); err != nil {
		return "", nil, domain.ErrUnauthorized
	}

	token, err := s.session.Create(user)
	if err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}

	return token, user, nil
}
