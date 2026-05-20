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

const defaultListLimit = 20

type ListParams struct {
	Limit    int
	Offset   int
	Sort     domain.SortParams
	Query    string
	IssuedBy string // optional narrow to a specific issuer's tokens
}

type ListResult struct {
	Tokens []*domain.Token
	Total  int
	Limit  int
	Offset int
}

// List returns tokens the authenticated caller can see, scoped by the
// namespaces they can read.
//
// Flow (EL-4 §7, T5.5):
//  1. effective = pdp.EffectiveDomains(caller, "namespace", "read"); if empty,
//     return an empty list (acceptance: empty → empty list, not 403).
//  2. Build a TokenFilter from the effective scope plus optional IssuedBy /
//     search and forward it to the repo. The repo applies the intersect
//     filter, sort, and pagination — no post-fetch loop here.
func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	nsScope := s.pdp.EffectiveDomains(claims.Email, domain.ObjectNamespace, domain.ActionRead)
	if nsScope.IsEmpty() {
		return &ListResult{
			Tokens: []*domain.Token{},
			Total:  0,
			Limit:  limit,
			Offset: params.Offset,
		}, nil
	}

	filter := domain.TokenFilter{
		NamespaceScopes: nsScope.Explicit,
		AnyNamespace:    nsScope.Wildcard,
		IssuedBy:        params.IssuedBy,
		Search:          params.Query,
	}
	repoParams := domain.TokenListParams{
		Limit:  limit,
		Offset: params.Offset,
		Sort:   params.Sort,
	}

	tokens, total, err := s.store.List(ctx, filter, repoParams)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}

	return &ListResult{
		Tokens: tokens,
		Total:  total,
		Limit:  limit,
		Offset: params.Offset,
	}, nil
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
