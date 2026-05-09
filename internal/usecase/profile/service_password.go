package profile

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) ChangePassword(ctx context.Context, currentPassword, newPassword string) (string, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return "", domain.ErrUnauthorized
	}

	user, err := s.users.Get(ctx, claims.Email)
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}

	if !claims.PasswordChangeRequired {
		if err := auth.VerifyPassword(user.PasswordHash, currentPassword); err != nil {
			return "", domain.ErrUnauthorized
		}
	}

	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	if err := s.pass.SetPassword(ctx, claims.Email, newHash, false); err != nil {
		return "", fmt.Errorf("set password: %w", err)
	}

	user.PasswordChangeRequired = false
	token, err := s.session.Create(user)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return token, nil
}
