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

	// Role-scope invariant: a token cannot grant more than its creator has on
	// each namespace. A reader-role token requires (Config, Read, ns); a writer
	// requires (Config, Write, ns). Handler already verified (Token, Create, ns);
	// this is the analogue of UpdateGroup's diff-boundary check.
	action, err := configActionForRole(in.Role)
	if err != nil {
		return nil, "", err
	}
	for _, ns := range in.Namespaces {
		if !s.pdp.Has(claims.Email, domain.Permission{
			Object: domain.ObjectConfig,
			Action: action,
			Domain: ns,
		}) {
			return nil, "", domain.ErrPermissionEscalation
		}
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

// configActionForRole maps a token role to the (Config, action) the creator
// must hold on each requested namespace.
func configActionForRole(role string) (string, error) {
	switch role {
	case domain.RoleReader:
		return domain.ActionRead, nil
	case domain.RoleWriter:
		return domain.ActionWrite, nil
	default:
		return "", domain.NewValidationError("role", "must be reader or writer")
	}
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
// namespaces in which they hold (Token, Read).
//
// Token visibility is gated by Token:Read per namespace (NOT Namespace:Read);
// see EL-4 T9.6.
func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	scope := s.pdp.EffectiveDomains(claims.Email, domain.ObjectToken, domain.ActionRead)
	if scope.IsEmpty() {
		return &ListResult{
			Tokens: []*domain.Token{},
			Total:  0,
			Limit:  limit,
			Offset: params.Offset,
		}, nil
	}

	filter := domain.TokenFilter{
		NamespaceScopes: scope.Explicit,
		AnyNamespace:    scope.Wildcard,
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

// Get returns the token if the caller holds (Token, Read) on at least one of
// the token's scoped namespaces. No ownership bypass: tokens are service
// credentials, not user-owned resources.
func (s *Service) Get(ctx context.Context, id string) (*domain.Token, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	token, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	for _, ns := range token.Namespaces {
		if s.pdp.Has(claims.Email, domain.Permission{
			Object: domain.ObjectToken,
			Action: domain.ActionRead,
			Domain: ns,
		}) {
			return token, nil
		}
	}

	return nil, domain.ErrForbidden
}

// Revoke deletes the token if the caller holds (Token, Write) on at least one
// of the token's scoped namespaces. No ownership bypass.
func (s *Service) Revoke(ctx context.Context, id string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	token, err := s.store.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get token for revocation: %w", err)
	}

	allowed := false

	for _, ns := range token.Namespaces {
		if s.pdp.Has(claims.Email, domain.Permission{
			Object: domain.ObjectToken,
			Action: domain.ActionWrite,
			Domain: ns,
		}) {
			allowed = true

			break
		}
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	return nil
}
