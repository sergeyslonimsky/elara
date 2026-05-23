package user

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// ResetPassword sets a new password for another user. Authorization
// `(User, Write, *)` is enforced in the handler (EL-4 M9).
func (s *Service) ResetPassword(ctx context.Context, targetEmail, newPassword string) error {
	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.store.SetPassword(ctx, targetEmail, newHash, true); err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	return nil
}
