package token

//go:generate mockgen -destination=mocks/service_mock.go -package=token_mock -source=service.go

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

const (
	tokenPrefix    = "elara_"
	tokenRandBytes = 32

	// maxUsernameAttempts bounds the regenerate-on-collision loop. With a
	// 30-bit suffix a single collision is already improbable; five in a row
	// means something is wrong (e.g. a broken random source).
	maxUsernameAttempts = 5
)

type (
	pdp interface {
		Has(principal string, perm domain.Permission) bool
		HasNamespace(actor, name string, action domain.Action) bool
		EffectiveDomains(
			principal string,
			object domain.Object,
			action domain.Action,
		) authz.DomainSet
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
		GetByUsername(ctx context.Context, username string) (*domain.Token, error)
		SetUsername(ctx context.Context, id, username string) error
		ListAll(ctx context.Context) ([]*domain.Token, error)
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
	bytes := make([]byte, tokenRandBytes)

	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("generate token bytes: %w", err)
	}

	raw := tokenPrefix + base64.RawURLEncoding.EncodeToString(bytes)

	return raw, HashRawToken(raw), nil
}

// HashRawToken returns the SHA-256 hex digest under which a raw token is
// stored. Exported so the etcd interceptor hashes exactly the same way.
func HashRawToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:])
}

// generateUsernameSuffix returns TokenUsernameSuffixLen lowercase base32
// characters (a-z, 2-7) — about 30 bits, enough that collisions are a
// retry, not a design concern.
func generateUsernameSuffix() string {
	return strings.ToLower(rand.Text()[:domain.TokenUsernameSuffixLen])
}
