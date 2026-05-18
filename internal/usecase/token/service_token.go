package token

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type CreateInput struct {
	Name       string
	Namespaces []string
	Role       string
	ExpiresAt  *time.Time
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.Token, string, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, "", domain.ErrUnauthorized
	}

	if len(in.Namespaces) == 0 {
		return nil, "", domain.NewValidationError("namespaces", "at least one namespace is required")
	}

	if err := s.checkNamespaceAccess(ctx, in.Namespaces, in.Role); err != nil {
		return nil, "", err
	}

	rawToken, tokenHash, err := generateRawToken()
	if err != nil {
		return nil, "", err
	}

	token := &domain.Token{
		ID:         uuid.New().String(),
		IssuedBy:   claims.Email,
		Name:       in.Name,
		TokenHash:  tokenHash,
		Namespaces: in.Namespaces,
		Role:       in.Role,
		ExpiresAt:  in.ExpiresAt,
		CreatedAt:  time.Now().UTC(),
	}

	if err := token.Validate(); err != nil {
		return nil, "", fmt.Errorf("validate: %w", err)
	}

	if err = s.store.Create(ctx, token); err != nil {
		return nil, "", fmt.Errorf("create token: %w", err)
	}

	return token, rawToken, nil
}

func (s *Service) List(ctx context.Context, issuedBy string) ([]*domain.Token, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	isAdmin, err := s.enforcer.Enforce(claims.Email, domain.DomainAll, domain.ObjectToken, domain.ActionRead)
	if err != nil {
		return nil, fmt.Errorf("enforce admin token read: %w", err)
	}

	tokens, err := s.store.List(ctx, issuedBy)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}

	if isAdmin {
		return tokens, nil
	}

	return s.filterTokensForCaller(claims.Email, tokens)
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Token, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	token, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	if token.IssuedBy == claims.Email {
		return token, nil
	}

	isAdmin, err := s.enforcer.Enforce(claims.Email, domain.DomainAll, domain.ObjectToken, domain.ActionRead)
	if err != nil {
		return nil, fmt.Errorf("enforce admin token get: %w", err)
	}

	if isAdmin {
		return token, nil
	}

	// Check if caller has access to any of the token's namespaces.
	for _, ns := range token.Namespaces {
		allowed, err := s.enforcer.Enforce(claims.Email, ns, domain.ObjectNamespace, domain.ActionRead)
		if err != nil {
			return nil, fmt.Errorf("enforce namespace read: %w", err)
		}

		if allowed {
			return token, nil
		}
	}

	return nil, domain.ErrForbidden
}

func (s *Service) Revoke(ctx context.Context, id string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	token, err := s.store.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get token for revocation: %w", err)
	}

	allowed := token.IssuedBy == claims.Email
	if !allowed {
		isAdmin, err := s.enforcer.Enforce(claims.Email, domain.DomainAll, domain.ObjectToken, domain.ActionWrite)
		if err != nil {
			return fmt.Errorf("enforce admin token write: %w", err)
		}
		allowed = isAdmin
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	return nil
}

// checkNamespaceAccess verifies the caller can read each namespace and, for
// writer tokens, can also write configs in each namespace.
func (s *Service) checkNamespaceAccess(ctx context.Context, namespaces []string, role string) error {
	for _, ns := range namespaces {
		if err := auth.CheckAccess(ctx, s.enforcer, ns, domain.ObjectNamespace, domain.ActionRead); err != nil {
			return fmt.Errorf("check access: %w", err)
		}
	}

	if role != "writer" {
		return nil
	}

	for _, ns := range namespaces {
		if err := auth.CheckAccess(ctx, s.enforcer, ns, domain.ObjectConfig, domain.ActionWrite); err != nil {
			return fmt.Errorf("check access: %w", err)
		}
	}

	return nil
}

// filterTokensForCaller returns only tokens the non-admin caller is allowed to see:
// their own tokens plus any token scoped to a namespace they can read.
func (s *Service) filterTokensForCaller(
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

		canSee, err := s.canSeeToken(callerEmail, t.Namespaces, nsCache)
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
func (s *Service) canSeeToken(
	callerEmail string,
	namespaces []string,
	nsCache map[string]bool,
) (bool, error) {
	for _, ns := range namespaces {
		allowed, cached := nsCache[ns]
		if !cached {
			var err error

			allowed, err = s.enforcer.Enforce(callerEmail, ns, domain.ObjectNamespace, domain.ActionRead)
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
