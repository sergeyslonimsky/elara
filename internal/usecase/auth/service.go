package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=auth_mock -source=service.go

const tokenBytes = 16

type (
	oidcProvider interface {
		AuthURL(state, nonce string) string
		Exchange(ctx context.Context, code, nonce string) (*auth.Identity, error)
	}

	userStore interface {
		Get(ctx context.Context, email string) (*domain.User, error)
		Upsert(ctx context.Context, user *domain.User) error
	}

	sessionCreator interface {
		Create(user *domain.User) (string, error)
	}

	bootstrapEnforcer interface {
		GetRolesForUser(user, domain string) ([]string, error)
		AddRoleForUser(user, role, domain string) error
	}
)

type Service struct {
	provider   oidcProvider
	users      userStore
	session    sessionCreator
	enforcer   bootstrapEnforcer
	adminEmail string
}

func New(
	provider oidcProvider,
	users userStore,
	session sessionCreator,
	enforcer bootstrapEnforcer,
	adminEmail string,
) *Service {
	return &Service{
		provider:   provider,
		users:      users,
		session:    session,
		enforcer:   enforcer,
		adminEmail: adminEmail,
	}
}

func randomToken() (string, error) {
	b := make([]byte, tokenBytes)

	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}
