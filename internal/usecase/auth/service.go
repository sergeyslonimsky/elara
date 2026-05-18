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

	// adminBootstrap is implemented by *auth.AdminBootstrap. It owns the
	// system admins group and writes membership g-rules; the auth usecase
	// asks it to ensure the configured bootstrap admin is a member after a
	// successful login.
	adminBootstrap interface {
		EnsureMember(ctx context.Context, email string) error
	}
)

type Service struct {
	provider   oidcProvider
	users      userStore
	session    sessionCreator
	admin      adminBootstrap
	adminEmail string
}

func New(
	provider oidcProvider,
	users userStore,
	session sessionCreator,
	admin adminBootstrap,
	adminEmail string,
) *Service {
	return &Service{
		provider:   provider,
		users:      users,
		session:    session,
		admin:      admin,
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
