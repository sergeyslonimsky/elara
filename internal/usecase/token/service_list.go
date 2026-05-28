package token

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const defaultListLimit = 20

type ListParams struct {
	Limit       int
	Offset      int
	Sort        domain.SortParams
	QueryParams []string
	IssuedBy    []string // optional narrow to specific issuers
	Namespaces  []string // optional narrow to tokens granting these namespaces
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
func (s *Service) List(ctx context.Context, user domain.AuthInfo, params ListParams) (*ListResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	scope := s.pdp.EffectiveDomains(user.Email, domain.ObjectToken, domain.ActionRead)
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
		Namespaces:      params.Namespaces,
		QueryParams:     params.QueryParams,
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
