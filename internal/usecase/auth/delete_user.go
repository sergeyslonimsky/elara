package auth

//go:generate mockgen -destination=mocks/mock_delete_user.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth deleteUserEnforcer,userGetterDeleter

import (
	"context"
	"fmt"

	internalauth "github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type deleteUserEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
	GetGroupingPolicy() [][]string
	DeleteUser(email string) error
}

type userGetterDeleter interface {
	Get(ctx context.Context, email string) (*domain.User, error)
	Delete(ctx context.Context, email string) error
}

// DeleteUserUseCase hard-deletes a basic-auth user and cleans up their Casbin rules.
type DeleteUserUseCase struct {
	enforcer deleteUserEnforcer
	users    userGetterDeleter
}

// NewDeleteUserUseCase returns a new DeleteUserUseCase.
func NewDeleteUserUseCase(enforcer deleteUserEnforcer, users userGetterDeleter) *DeleteUserUseCase {
	return &DeleteUserUseCase{enforcer: enforcer, users: users}
}

// Execute deletes the target user after validating self-deletion and last-admin guards.
func (uc *DeleteUserUseCase) Execute(ctx context.Context, targetEmail string) error {
	claims, ok := internalauth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	if err := uc.authorize(claims.Email); err != nil {
		return err
	}

	if claims.Email == targetEmail {
		return domain.NewValidationError("email", "cannot delete your own account")
	}

	if _, err := uc.users.Get(ctx, targetEmail); err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	if err := uc.validateLastAdmin(targetEmail); err != nil {
		return err
	}

	if err := uc.users.Delete(ctx, targetEmail); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if err := uc.enforcer.DeleteUser(targetEmail); err != nil {
		return fmt.Errorf("delete casbin user: %w", err)
	}

	return nil
}

func (uc *DeleteUserUseCase) authorize(callerEmail string) error {
	allowed, err := uc.enforcer.Enforce(
		callerEmail,
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

	return nil
}

func (uc *DeleteUserUseCase) validateLastAdmin(targetEmail string) error {
	rules := uc.enforcer.GetGroupingPolicy()
	adminCount := 0
	isTargetAdmin := false

	for _, rule := range rules {
		if len(rule) == 3 && rule[1] == internalauth.RoleAdmin && rule[2] == internalauth.ObjectAll {
			adminCount++
			if rule[0] == targetEmail {
				isTargetAdmin = true
			}
		}
	}

	if isTargetAdmin && adminCount == 1 {
		return domain.NewValidationError("email", "cannot delete the last admin")
	}

	return nil
}
