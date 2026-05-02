package auth

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/me_mock.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth roleGetter

type roleGetter interface {
	GetRolesForUser(user, domain string) ([]string, error)
}

// MeUseCase returns the current authenticated user's identity and roles.
type MeUseCase struct {
	enforcer roleGetter
}

// NewMeUseCase returns a MeUseCase backed by the given role getter.
func NewMeUseCase(enforcer roleGetter) *MeUseCase {
	return &MeUseCase{enforcer: enforcer}
}

// Execute extracts claims from the context and returns the user with their roles.
func (uc *MeUseCase) Execute(ctx context.Context) (*domain.User, []string, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, nil, domain.ErrUnauthorized
	}

	roles, err := uc.enforcer.GetRolesForUser(claims.Email, "*")
	if err != nil {
		return nil, nil, fmt.Errorf("get roles: %w", err)
	}

	user := &domain.User{
		Email: claims.Email,
		Name:  claims.Name,
	}

	return user, roles, nil
}
