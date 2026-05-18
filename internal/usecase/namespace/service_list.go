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

// List returns the namespaces visible to the authenticated caller, annotated
// with per-namespace permission flags the UI uses to gate actions.
//
// Authorization for the call itself is enforced at the handler — every
// authenticated user may invoke List. The two ways permissions enter this
// method are pure business logic:
//
//   - filterAccessible drops namespaces the caller cannot read.
//   - per-namespace CanWrite is computed for the remaining items so the UI
//     can show or hide write-only actions (e.g. "Import to this namespace").
func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		// Defence in depth — handler should already have rejected
		// unauthenticated calls via auth.RequireAuthenticated.
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

	s.annotatePermissions(claims.Email, paginated)

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
		if s.pdp.Has(callerEmail, domain.Permission{
			Object: domain.ObjectNamespace,
			Action: domain.ActionRead,
			Domain: ns.Name,
		}) {
			accessible = append(accessible, ns)
		}
	}

	return accessible
}

// annotatePermissions fills CanRead/CanWrite on each namespace from the
// caller's perspective. CanRead is implicitly true (the slice has already
// been filtered through filterAccessible); CanWrite is the result of a
// separate per-namespace check against config/write — that's the permission
// needed to import configs into the namespace.
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
