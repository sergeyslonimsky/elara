package auth

import (
	"context"
	"fmt"

	internalauth "github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/create_user_mock.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth createUserEnforcer,userCreator

type createUserEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type userCreator interface {
	Upsert(ctx context.Context, user *domain.User) error
	SetPassword(ctx context.Context, email, hash string, changeRequired bool) error
}

// CreateUserUseCase handles the creation of a new basic-auth user.
type CreateUserUseCase struct {
	enforcer createUserEnforcer
	users    userCreator
}

// NewCreateUserUseCase returns a CreateUserUseCase wired with all required dependencies.
func NewCreateUserUseCase(enforcer createUserEnforcer, users userCreator) *CreateUserUseCase {
	return &CreateUserUseCase{
		enforcer: enforcer,
		users:    users,
	}
}

// Execute creates a new user and sets their initial password with PasswordChangeRequired=true.
func (uc *CreateUserUseCase) Execute(ctx context.Context, email, name, initialPassword string) (*domain.User, error) {
	claims, ok := internalauth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(
		claims.Email,
		internalauth.ObjectAll,
		internalauth.ObjectUser,
		internalauth.ActionWrite,
	)
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	user := &domain.User{
		Email:    email,
		Name:     name,
		Provider: domain.ProviderBasicAuth,
	}

	if initialPassword == "" {
		user.Provider = domain.ProviderOIDC
	}

	if err := user.Validate(); err != nil {
		return nil, fmt.Errorf("validate user: %w", err)
	}

	if err := uc.users.Upsert(ctx, user); err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	if initialPassword != "" {
		hash, err := internalauth.HashPassword(initialPassword)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}

		if err := uc.users.SetPassword(ctx, email, hash, true); err != nil {
			return nil, fmt.Errorf("set password: %w", err)
		}
	}

	return user, nil
}
