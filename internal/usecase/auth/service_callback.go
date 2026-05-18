package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Callback handles the OIDC callback: exchanges the code for an identity,
// upserts the user, bootstraps the configured admin into the admins group
// when applicable, and creates a session token.
func (s *Service) Callback(ctx context.Context, code, nonce string) (string, *domain.User, error) {
	identity, err := s.provider.Exchange(ctx, code, nonce)
	if err != nil {
		return "", nil, fmt.Errorf("exchange code: %w", err)
	}

	user := &domain.User{
		Email:       identity.Email,
		Name:        identity.Name,
		Picture:     identity.Picture,
		Provider:    domain.ProviderOIDC,
		LastLoginAt: time.Now(),
	}

	if err = s.users.Upsert(ctx, user); err != nil {
		return "", nil, fmt.Errorf("upsert user: %w", err)
	}

	if identity.Email == s.adminEmail {
		if err = s.admin.EnsureMember(ctx, identity.Email); err != nil {
			return "", nil, fmt.Errorf("bootstrap admin: %w", err)
		}
	}

	token, err := s.session.Create(user)
	if err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}

	return token, user, nil
}
