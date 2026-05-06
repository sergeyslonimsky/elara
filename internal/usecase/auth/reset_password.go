package auth

import (
	"context"
	"fmt"

	internalauth "github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/reset_password_mock.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth resetPasswordEnforcer

type resetPasswordEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

// ResetPasswordUseCase allows an administrator to reset a user's password.
type ResetPasswordUseCase struct {
	enforcer resetPasswordEnforcer
	writer   passwordWriter
}

// NewResetPasswordUseCase returns a ResetPasswordUseCase wired with all required dependencies.
func NewResetPasswordUseCase(enforcer resetPasswordEnforcer, writer passwordWriter) *ResetPasswordUseCase {
	return &ResetPasswordUseCase{
		enforcer: enforcer,
		writer:   writer,
	}
}

// Execute resets the target user's password and forces a change on next login.
func (uc *ResetPasswordUseCase) Execute(ctx context.Context, targetEmail, newPassword string) error {
	claims, ok := internalauth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(
		claims.Email,
		internalauth.ObjectAll,
		internalauth.ObjectUser,
		internalauth.ActionWrite,
	)
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	newHash, err := internalauth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := uc.writer.SetPassword(ctx, targetEmail, newHash, true); err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	return nil
}
