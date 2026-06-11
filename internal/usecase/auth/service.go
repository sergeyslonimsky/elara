package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=auth_mock -source=service.go

const tokenBytes = 16

type (
	oidcProvider interface {
		AuthURL(state, nonce string) string
		Exchange(ctx context.Context, code, nonce string) (*auth.Identity, error)
	}

	// userStore is the user-service surface the auth usecase composes inside
	// its login flows. It mirrors a subset of *auth.UserService — narrow on
	// purpose so the auth-flow tests can fake user mutations without dragging
	// the full service surface into expectations.
	userStore interface {
		GetByIdentity(ctx context.Context, provider, subject string) (*domain.User, error)
		GetByEmail(ctx context.Context, email string) (*domain.User, error)
		LinkIdentity(ctx context.Context, userID uuid.UUID, identity domain.Identity) (*domain.User, error)
		RecordLogin(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	}

	sessionsService interface {
		Create(ctx context.Context, params sessions.CreateParams) (*domain.Session, error)
	}
)

type Service struct {
	txm      storage.Manager
	provider oidcProvider
	users    userStore
	sessions sessionsService
}

// New constructs the auth Service.
func New(
	txm storage.Manager,
	provider oidcProvider,
	users userStore,
	sessionsSvc sessionsService,
) *Service {
	return &Service{
		txm:      txm,
		provider: provider,
		users:    users,
		sessions: sessionsSvc,
	}
}

func randomToken() (string, error) {
	b := make([]byte, tokenBytes)

	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}
