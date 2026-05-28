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
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

const (
	tokenPrefix    = "elara_"
	tokenRandBytes = 32
)

type (
	pdp interface {
		Has(principal string, perm domain.Permission) bool
		EffectiveDomains(principal string, object domain.Object, action domain.Action) authz.DomainSet
	}

	store interface {
		Create(ctx context.Context, token *domain.Token) error
		List(
			ctx context.Context,
			filter domain.TokenFilter,
			params domain.TokenListParams,
		) ([]*domain.Token, int, error)
		Delete(ctx context.Context, id string) error
		GetByID(ctx context.Context, id string) (*domain.Token, error)
	}
)

type Service struct {
	pdp   pdp
	store store
}

func New(pdp pdp, store store) *Service {
	return &Service{
		pdp:   pdp,
		store: store,
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
