package auth

//go:generate mockgen -destination=mocks/mock_token.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth tokenCreator,tokenLister,tokenDeleter,tokenIDGetter

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

const (
	patPrefix    = "elara_"
	patRandBytes = 32
)

type tokenCreator interface {
	Create(ctx context.Context, pat *domain.PAT) error
}

type tokenLister interface {
	List(ctx context.Context, userEmail string) ([]*domain.PAT, error)
}

type tokenDeleter interface {
	Delete(ctx context.Context, id string) error
}

type tokenIDGetter interface {
	GetByID(ctx context.Context, id string) (*domain.PAT, error)
}

// CreateTokenUseCase creates a new Personal Access Token for the authenticated user.
type CreateTokenUseCase struct {
	tokens tokenCreator
}

// NewCreateTokenUseCase returns a new CreateTokenUseCase.
func NewCreateTokenUseCase(tokens tokenCreator) *CreateTokenUseCase {
	return &CreateTokenUseCase{tokens: tokens}
}

// Execute creates a PAT and returns both the stored PAT and the raw token string.
func (uc *CreateTokenUseCase) Execute(
	ctx context.Context,
	name string,
	namespaces []string,
	expiresAt *time.Time,
) (*domain.PAT, string, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, "", domain.ErrUnauthorized
	}

	rawToken, tokenHash, err := generateRawToken()
	if err != nil {
		return nil, "", err
	}

	pat := &domain.PAT{
		ID:         uuid.New().String(),
		UserEmail:  claims.Email,
		Name:       name,
		TokenHash:  tokenHash,
		Namespaces: namespaces,
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now().UTC(),
	}

	if err = uc.tokens.Create(ctx, pat); err != nil {
		return nil, "", fmt.Errorf("create token: %w", err)
	}

	return pat, rawToken, nil
}

// ListTokensUseCase returns PATs filtered by user email.
type ListTokensUseCase struct {
	tokens tokenLister
}

// NewListTokensUseCase returns a new ListTokensUseCase.
func NewListTokensUseCase(tokens tokenLister) *ListTokensUseCase {
	return &ListTokensUseCase{tokens: tokens}
}

// Execute returns all tokens for the given user email (empty = all tokens).
func (uc *ListTokensUseCase) Execute(ctx context.Context, userEmail string) ([]*domain.PAT, error) {
	tokens, err := uc.tokens.List(ctx, userEmail)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}

	return tokens, nil
}

// GetTokenUseCase returns a single token by ID.
type GetTokenUseCase struct {
	tokens tokenIDGetter
}

// NewGetTokenUseCase returns a new GetTokenUseCase.
func NewGetTokenUseCase(tokens tokenIDGetter) *GetTokenUseCase {
	return &GetTokenUseCase{tokens: tokens}
}

// Execute returns the token with the given ID.
func (uc *GetTokenUseCase) Execute(ctx context.Context, id string) (*domain.PAT, error) {
	pat, err := uc.tokens.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	return pat, nil
}

// RevokeTokenUseCase deletes a token by ID.
type RevokeTokenUseCase struct {
	tokens tokenDeleter
}

// NewRevokeTokenUseCase returns a new RevokeTokenUseCase.
func NewRevokeTokenUseCase(tokens tokenDeleter) *RevokeTokenUseCase {
	return &RevokeTokenUseCase{tokens: tokens}
}

// Execute deletes the token with the given ID.
func (uc *RevokeTokenUseCase) Execute(ctx context.Context, id string) error {
	if err := uc.tokens.Delete(ctx, id); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	return nil
}

func generateRawToken() (string, string, error) {
	b := make([]byte, patRandBytes)

	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token bytes: %w", err)
	}

	raw := patPrefix + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))

	return raw, hex.EncodeToString(sum[:]), nil
}
