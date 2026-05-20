package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

const defaultListLimit = 20

type ListParams struct {
	Limit  int
	Offset int
	Sort   domain.SortParams
	Query  string
}

type ListResult struct {
	Namespaces []*domain.Namespace
	Total      int
	Limit      int
	Offset     int
}

// List returns namespaces the authenticated caller can read, with per-item
// CanRead/CanWrite flags for the UI.
//
// Filtering happens at the repository: the caller's effective namespace set
// (from PDP.EffectiveDomains) is translated into a NamespaceFilter — no
// post-fetch pdp.Has loop. An empty effective set returns an empty list, not
// an error (EL-4 §7 acceptance: empty responses → empty list, not 403).
func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	scope := s.pdp.EffectiveDomains(claims.Email, domain.ObjectNamespace, domain.ActionRead)
	if scope.IsEmpty() {
		return &ListResult{
			Namespaces: []*domain.Namespace{},
			Total:      0,
			Limit:      limit,
			Offset:     params.Offset,
		}, nil
	}

	filter := domain.NamespaceFilter{
		Names:    scope.Explicit,
		Wildcard: scope.Wildcard,
		Search:   params.Query,
	}

	repoParams := domain.NamespaceListParams{
		Limit:  limit,
		Offset: params.Offset,
		Sort:   params.Sort,
	}

	namespaces, total, err := s.store.List(ctx, filter, repoParams)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	if err := s.populateConfigCounts(ctx, namespaces); err != nil {
		return nil, err
	}

	s.annotatePermissions(claims.Email, namespaces)

	return &ListResult{
		Namespaces: namespaces,
		Total:      total,
		Limit:      limit,
		Offset:     params.Offset,
	}, nil
}

// annotatePermissions fills CanRead/CanWrite from the caller's perspective.
// CanRead is implicitly true (the list is already scoped to readable
// namespaces); CanWrite gates the "Import here" action and requires
// config/write on the namespace.
func (s *Service) annotatePermissions(callerEmail string, namespaces []*domain.Namespace) {
	for _, ns := range namespaces {
		ns.CanRead = true
		ns.CanWrite = s.pdp.Has(callerEmail, domain.Permission{
			Object: domain.ObjectConfig,
			Action: domain.ActionWrite,
			Domain: ns.Name,
		})
	}
}
