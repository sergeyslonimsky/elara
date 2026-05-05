package auth

//go:generate mockgen -destination=mocks/mock_user.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth userLister,userGetter

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type userEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type userLister interface {
	List(ctx context.Context) ([]*domain.User, error)
}

type userGetter interface {
	Get(ctx context.Context, email string) (*domain.User, error)
}

// ListUsersUseCase returns all registered users.
type ListUsersUseCase struct {
	enforcer userEnforcer
	users    userLister
}

// NewListUsersUseCase returns a new ListUsersUseCase.
func NewListUsersUseCase(enforcer userEnforcer, users userLister) *ListUsersUseCase {
	return &ListUsersUseCase{enforcer: enforcer, users: users}
}

// Execute returns all users.
func (uc *ListUsersUseCase) Execute(ctx context.Context) ([]*domain.User, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, "*", "user", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	users, err := uc.users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return users, nil
}

// GetUserUseCase returns a single user by email.
type GetUserUseCase struct {
	enforcer userEnforcer
	users    userGetter
}

// NewGetUserUseCase returns a new GetUserUseCase.
func NewGetUserUseCase(enforcer userEnforcer, users userGetter) *GetUserUseCase {
	return &GetUserUseCase{enforcer: enforcer, users: users}
}

// Execute returns the user with the given email.
func (uc *GetUserUseCase) Execute(ctx context.Context, email string) (*domain.User, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, "*", "user", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	user, err := uc.users.Get(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}
