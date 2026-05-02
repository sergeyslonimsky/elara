package auth

//go:generate mockgen -destination=mocks/mock_user.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth userLister,userGetter

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type userLister interface {
	List(ctx context.Context) ([]*domain.User, error)
}

type userGetter interface {
	Get(ctx context.Context, email string) (*domain.User, error)
}

// ListUsersUseCase returns all registered users.
type ListUsersUseCase struct {
	users userLister
}

// NewListUsersUseCase returns a new ListUsersUseCase.
func NewListUsersUseCase(users userLister) *ListUsersUseCase {
	return &ListUsersUseCase{users: users}
}

// Execute returns all users.
func (uc *ListUsersUseCase) Execute(ctx context.Context) ([]*domain.User, error) {
	users, err := uc.users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return users, nil
}

// GetUserUseCase returns a single user by email.
type GetUserUseCase struct {
	users userGetter
}

// NewGetUserUseCase returns a new GetUserUseCase.
func NewGetUserUseCase(users userGetter) *GetUserUseCase {
	return &GetUserUseCase{users: users}
}

// Execute returns the user with the given email.
func (uc *GetUserUseCase) Execute(ctx context.Context, email string) (*domain.User, error) {
	user, err := uc.users.Get(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}
