package user

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) ResetPassword(ctx context.Context, targetEmail, newPassword string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(
		claims.Email,
		auth.ObjectAll,
		auth.ObjectUser,
		auth.ActionWrite,
	)
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.store.SetPassword(ctx, targetEmail, newHash, true); err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	return nil
}
