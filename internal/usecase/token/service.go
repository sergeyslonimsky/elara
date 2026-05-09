package token

//go:generate mockgen -destination=mocks/service_mock.go -package=token_mock -source=service.go

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const (
	tokenPrefix    = "elara_"
	tokenRandBytes = 32
)

type (
	enforcer interface {
		Enforce(subject, domain, object, action string) (bool, error)
	}

	store interface {
		Create(ctx context.Context, token *domain.Token) error
		List(ctx context.Context, issuedBy string) ([]*domain.Token, error)
		Delete(ctx context.Context, id string) error
		GetByID(ctx context.Context, id string) (*domain.Token, error)
	}
)

type Service struct {
	enforcer enforcer
	store    store
}

func New(enforcer enforcer, store store) *Service {
	return &Service{
		enforcer: enforcer,
		store:    store,
	}
}

func generateRawToken() (string, string, error) {
	b := make([]byte, tokenRandBytes)

	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token bytes: %w", err)
	}

	raw := tokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))

	return raw, hex.EncodeToString(sum[:]), nil
}
