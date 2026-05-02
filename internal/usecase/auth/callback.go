package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/callback_mock.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth callbackProvider,userUpserter,policyLoader,sessionCreator

type callbackProvider interface {
	Exchange(ctx context.Context, code, nonce string) (*auth.Identity, error)
}

type userUpserter interface {
	Upsert(ctx context.Context, user *domain.User) error
}

type policyLoader interface {
	Load(ctx context.Context) ([][]string, error)
	Save(ctx context.Context, rules [][]string) error
}

type sessionCreator interface {
	Create(user *domain.User) (string, error)
}

// CallbackUseCase handles the OIDC callback: exchanges the code for an identity,
// upserts the user, bootstraps admin if needed, and creates a session token.
type CallbackUseCase struct {
	provider    callbackProvider
	users       userUpserter
	session     sessionCreator
	enforcer    casbin.BootstrapEnforcer
	adminEmails []string
	loader      policyLoader
}

// NewCallbackUseCase returns a CallbackUseCase wired with all required dependencies.
func NewCallbackUseCase(
	provider callbackProvider,
	users userUpserter,
	session sessionCreator,
	enforcer casbin.BootstrapEnforcer,
	loader policyLoader,
	adminEmails []string,
) *CallbackUseCase {
	return &CallbackUseCase{
		provider:    provider,
		users:       users,
		session:     session,
		enforcer:    enforcer,
		loader:      loader,
		adminEmails: adminEmails,
	}
}

// Execute exchanges the authorization code for an identity, upserts the user,
// optionally bootstraps admin role, and returns a signed session token.
func (uc *CallbackUseCase) Execute(ctx context.Context, code, nonce string) (string, *domain.User, error) {
	identity, err := uc.provider.Exchange(ctx, code, nonce)
	if err != nil {
		return "", nil, fmt.Errorf("exchange code: %w", err)
	}

	user := &domain.User{
		Email:       identity.Email,
		Name:        identity.Name,
		Picture:     identity.Picture,
		Provider:    "oidc",
		LastLoginAt: time.Now(),
	}

	if err = uc.users.Upsert(ctx, user); err != nil {
		return "", nil, fmt.Errorf("upsert user: %w", err)
	}

	if err = casbin.CheckBootstrapAdmin(ctx, identity.Email, uc.adminEmails, uc.enforcer, uc.loader); err != nil {
		return "", nil, fmt.Errorf("bootstrap admin: %w", err)
	}

	token, err := uc.session.Create(user)
	if err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}

	return token, user, nil
}
