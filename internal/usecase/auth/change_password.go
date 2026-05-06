package auth

import (
	"context"
	"fmt"

	internalauth "github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/change_password_mock.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth passwordReader,passwordWriter

type passwordReader interface {
	Get(ctx context.Context, email string) (*domain.User, error)
}

type passwordWriter interface {
	SetPassword(ctx context.Context, email, hash string, changeRequired bool) error
}

// ChangePasswordUseCase handles user password changes.
type ChangePasswordUseCase struct {
	reader passwordReader
	writer passwordWriter
}

// NewChangePasswordUseCase returns a ChangePasswordUseCase wired with all required dependencies.
func NewChangePasswordUseCase(reader passwordReader, writer passwordWriter) *ChangePasswordUseCase {
	return &ChangePasswordUseCase{
		reader: reader,
		writer: writer,
	}
}

// Execute updates the user's password. If PasswordChangeRequired is false in claims,
// it verifies the current password first.
func (uc *ChangePasswordUseCase) Execute(ctx context.Context, currentPassword, newPassword string) error {
	claims, ok := internalauth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	user, err := uc.reader.Get(ctx, claims.Email)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	if !claims.PasswordChangeRequired {
		if err := internalauth.VerifyPassword(user.PasswordHash, currentPassword); err != nil {
			return domain.ErrUnauthorized
		}
	}

	newHash, err := internalauth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := uc.writer.SetPassword(ctx, claims.Email, newHash, false); err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	return nil
}
