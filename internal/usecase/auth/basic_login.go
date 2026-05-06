package auth

import (
	"context"
	"fmt"

	internalauth "github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/basic_login_mock.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth basicAuthUserGetter

type basicAuthUserGetter interface {
	Get(ctx context.Context, email string) (*domain.User, error)
}

// BasicLoginUseCase handles authentication via email and password.
type BasicLoginUseCase struct {
	users      basicAuthUserGetter
	session    sessionCreator
	enforcer   casbin.BootstrapEnforcer
	adminEmail string
}

// NewBasicLoginUseCase returns a BasicLoginUseCase wired with all required dependencies.
func NewBasicLoginUseCase(
	users basicAuthUserGetter,
	session sessionCreator,
	enforcer casbin.BootstrapEnforcer,
	adminEmail string,
) *BasicLoginUseCase {
	return &BasicLoginUseCase{
		users:      users,
		session:    session,
		enforcer:   enforcer,
		adminEmail: adminEmail,
	}
}

// Execute verifies the user's credentials, optionally bootstraps admin role,
// and returns a signed session token.
func (uc *BasicLoginUseCase) Execute(ctx context.Context, email, password string) (string, *domain.User, error) {
	user, err := uc.users.Get(ctx, email)
	if err != nil {
		return "", nil, domain.ErrUnauthorized
	}

	if err := internalauth.VerifyPassword(user.PasswordHash, password); err != nil {
		return "", nil, domain.ErrUnauthorized
	}

	if err := casbin.CheckBootstrapAdmin(ctx, email, uc.adminEmail, uc.enforcer); err != nil {
		return "", nil, fmt.Errorf("bootstrap admin: %w", err)
	}

	token, err := uc.session.Create(user)
	if err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}

	return token, user, nil
}
