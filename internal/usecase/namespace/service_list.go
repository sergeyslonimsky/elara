package namespace

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/util/sliceutil"
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

func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	namespaces, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	namespaces = s.filterAccessible(claims.Email, namespaces)
	namespaces = filterByQuery(namespaces, params.Query)
	sortNamespaces(namespaces, params.Sort)

	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	total := len(namespaces)
	paginated := sliceutil.Paginate(namespaces, params.Offset, limit)

	if err := s.populateConfigCounts(ctx, paginated); err != nil {
		return nil, err
	}

	return &ListResult{
		Namespaces: paginated,
		Total:      total,
		Limit:      limit,
		Offset:     params.Offset,
	}, nil
}

// filterAccessible returns only namespaces the caller email can read.
func (s *Service) filterAccessible(callerEmail string, namespaces []*domain.Namespace) []*domain.Namespace {
	accessible := namespaces[:0]
	for _, ns := range namespaces {
		allowed, _ := s.enforcer.Enforce(callerEmail, ns.Name, auth.ObjectNamespace, auth.ActionRead)
		if allowed {
			accessible = append(accessible, ns)
		}
	}

	return accessible
}

// filterByQuery filters namespaces by case-insensitive substring match on name.
func filterByQuery(namespaces []*domain.Namespace, query string) []*domain.Namespace {
	if query == "" {
		return namespaces
	}

	queryLower := strings.ToLower(query)
	filtered := make([]*domain.Namespace, 0, len(namespaces))

	for _, ns := range namespaces {
		if strings.Contains(strings.ToLower(ns.Name), queryLower) {
			filtered = append(filtered, ns)
		}
	}

	return filtered
}

func sortNamespaces(namespaces []*domain.Namespace, params domain.SortParams) {
	sort.Slice(namespaces, func(i, j int) bool {
		a, b := namespaces[i], namespaces[j]

		var less bool

		switch params.Field {
		case "modified":
			less = a.UpdatedAt.Before(b.UpdatedAt)
		default:
			less = a.Name < b.Name
		}

		if params.Desc {
			return !less
		}

		return less
	})
}
