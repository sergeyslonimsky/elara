package auth

//go:generate mockgen -destination=mocks/mock_token.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth tokenCreator,tokenLister,tokenDeleter,tokenIDGetter,tokenEnforcer

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
	tokenPrefix    = "elara_"
	tokenRandBytes = 32
)

type tokenEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type tokenCreator interface {
	Create(ctx context.Context, token *domain.Token) error
}

type tokenLister interface {
	List(ctx context.Context, issuedBy string) ([]*domain.Token, error)
}

type tokenDeleter interface {
	Delete(ctx context.Context, id string) error
}

type tokenIDGetter interface {
	GetByID(ctx context.Context, id string) (*domain.Token, error)
}

// CreateTokenUseCase creates a new service credential (Token) for the authenticated user.
type CreateTokenUseCase struct {
	enforcer tokenEnforcer
	tokens   tokenCreator
}

// NewCreateTokenUseCase returns a new CreateTokenUseCase.
func NewCreateTokenUseCase(enforcer tokenEnforcer, tokens tokenCreator) *CreateTokenUseCase {
	return &CreateTokenUseCase{enforcer: enforcer, tokens: tokens}
}

// Execute creates a Token and returns both the stored Token and the raw token string.
func (uc *CreateTokenUseCase) Execute(
	ctx context.Context,
	name string,
	namespaces []string,
	role string,
	expiresAt *time.Time,
) (*domain.Token, string, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, "", domain.ErrUnauthorized
	}

	if len(namespaces) == 0 {
		return nil, "", domain.NewValidationError("namespaces", "at least one namespace is required")
	}

	if err := uc.checkNamespaceAccess(ctx, namespaces, role); err != nil {
		return nil, "", err
	}

	rawToken, tokenHash, err := generateRawToken()
	if err != nil {
		return nil, "", err
	}

	token := &domain.Token{
		ID:         uuid.New().String(),
		IssuedBy:   claims.Email,
		Name:       name,
		TokenHash:  tokenHash,
		Namespaces: namespaces,
		Role:       role,
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now().UTC(),
	}

	if err := token.Validate(); err != nil {
		return nil, "", fmt.Errorf("validate: %w", err)
	}

	if err = uc.tokens.Create(ctx, token); err != nil {
		return nil, "", fmt.Errorf("create token: %w", err)
	}

	return token, rawToken, nil
}

// checkNamespaceAccess verifies the caller can read each namespace and, for
// writer tokens, can also write configs in each namespace.
func (uc *CreateTokenUseCase) checkNamespaceAccess(ctx context.Context, namespaces []string, role string) error {
	for _, ns := range namespaces {
		if err := auth.CheckAccess(ctx, uc.enforcer, ns, auth.ObjectNamespace, auth.ActionRead); err != nil {
			return fmt.Errorf("check access: %w", err)
		}
	}

	if role != "writer" {
		return nil
	}

	for _, ns := range namespaces {
		if err := auth.CheckAccess(ctx, uc.enforcer, ns, auth.ObjectConfig, auth.ActionWrite); err != nil {
			return fmt.Errorf("check access: %w", err)
		}
	}

	return nil
}

// ListTokensUseCase returns Tokens filtered by issuer email.
type ListTokensUseCase struct {
	enforcer tokenEnforcer
	tokens   tokenLister
}

// NewListTokensUseCase returns a new ListTokensUseCase.
func NewListTokensUseCase(enforcer tokenEnforcer, tokens tokenLister) *ListTokensUseCase {
	return &ListTokensUseCase{enforcer: enforcer, tokens: tokens}
}

// Execute returns tokens for the given user email.
// Admins can see any user's tokens.
// Non-admins see tokens they issued OR tokens scoped to namespaces they can read.
func (uc *ListTokensUseCase) Execute(ctx context.Context, issuedBy string) ([]*domain.Token, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	isAdmin, err := uc.enforcer.Enforce(claims.Email, auth.ObjectAll, auth.ObjectToken, auth.ActionRead)
	if err != nil {
		return nil, fmt.Errorf("enforce admin token read: %w", err)
	}

	tokens, err := uc.tokens.List(ctx, issuedBy)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}

	if isAdmin {
		return tokens, nil
	}

	return uc.filterTokensForCaller(claims.Email, tokens)
}

// filterTokensForCaller returns only tokens the non-admin caller is allowed to see:
// their own tokens plus any token scoped to a namespace they can read.
func (uc *ListTokensUseCase) filterTokensForCaller(
	callerEmail string,
	tokens []*domain.Token,
) ([]*domain.Token, error) {
	var result []*domain.Token
	nsCache := make(map[string]bool)

	for _, t := range tokens {
		if t.IssuedBy == callerEmail {
			result = append(result, t)

			continue
		}

		canSee, err := uc.canSeeToken(callerEmail, t.Namespaces, nsCache)
		if err != nil {
			return nil, err
		}

		if canSee {
			result = append(result, t)
		}
	}

	return result, nil
}

// canSeeToken checks whether callerEmail can read any of the given namespaces,
// using nsCache to avoid redundant enforcer calls.
func (uc *ListTokensUseCase) canSeeToken(
	callerEmail string,
	namespaces []string,
	nsCache map[string]bool,
) (bool, error) {
	for _, ns := range namespaces {
		allowed, cached := nsCache[ns]
		if !cached {
			var err error

			allowed, err = uc.enforcer.Enforce(callerEmail, ns, auth.ObjectNamespace, auth.ActionRead)
			if err != nil {
				return false, fmt.Errorf("enforce namespace read: %w", err)
			}

			nsCache[ns] = allowed
		}

		if allowed {
			return true, nil
		}
	}

	return false, nil
}

// GetTokenUseCase returns a single token by ID with ownership or namespace check.
type GetTokenUseCase struct {
	enforcer tokenEnforcer
	tokens   tokenIDGetter
}

// NewGetTokenUseCase returns a new GetTokenUseCase.
func NewGetTokenUseCase(enforcer tokenEnforcer, tokens tokenIDGetter) *GetTokenUseCase {
	return &GetTokenUseCase{enforcer: enforcer, tokens: tokens}
}

// Execute returns the token with the given ID if the caller is an admin, the issuer,
// or has access to any of the token's namespaces.
func (uc *GetTokenUseCase) Execute(ctx context.Context, id string) (*domain.Token, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	token, err := uc.tokens.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	if token.IssuedBy == claims.Email {
		return token, nil
	}

	isAdmin, err := uc.enforcer.Enforce(claims.Email, auth.ObjectAll, auth.ObjectToken, auth.ActionRead)
	if err != nil {
		return nil, fmt.Errorf("enforce admin token get: %w", err)
	}

	if isAdmin {
		return token, nil
	}

	// Check if caller has access to any of the token's namespaces.
	for _, ns := range token.Namespaces {
		allowed, err := uc.enforcer.Enforce(claims.Email, ns, auth.ObjectNamespace, auth.ActionRead)
		if err != nil {
			return nil, fmt.Errorf("enforce namespace read: %w", err)
		}

		if allowed {
			return token, nil
		}
	}

	return nil, domain.ErrForbidden
}

// RevokeTokenUseCase deletes a token by ID with ownership check.
type RevokeTokenUseCase struct {
	enforcer tokenEnforcer
	tokens   tokenDeleter
	getter   tokenIDGetter
}

// NewRevokeTokenUseCase returns a new RevokeTokenUseCase.
func NewRevokeTokenUseCase(enforcer tokenEnforcer, tokens tokenDeleter, getter tokenIDGetter) *RevokeTokenUseCase {
	return &RevokeTokenUseCase{enforcer: enforcer, tokens: tokens, getter: getter}
}

// Execute deletes the token with the given ID if the caller is an admin or the issuer.
func (uc *RevokeTokenUseCase) Execute(ctx context.Context, id string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	token, err := uc.getter.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get token for revocation: %w", err)
	}

	allowed := token.IssuedBy == claims.Email
	if !allowed {
		isAdmin, err := uc.enforcer.Enforce(claims.Email, auth.ObjectAll, auth.ObjectToken, auth.ActionWrite)
		if err != nil {
			return fmt.Errorf("enforce admin token write: %w", err)
		}
		allowed = isAdmin
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := uc.tokens.Delete(ctx, id); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	return nil
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
