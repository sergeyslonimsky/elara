package namespace

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_list.go -package=mock_namespace . listEnforcer,nsLister,listConfigCounter

const defaultListLimit = 20

type listEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type nsLister interface {
	List(ctx context.Context) ([]*domain.Namespace, error)
}

type listConfigCounter interface {
	CountConfigs(ctx context.Context, name string) (int, error)
}

type NSListParams struct {
	Limit  int
	Offset int
	Sort   domain.SortParams
	Query  string
}

type NSListResult struct {
	Namespaces []*domain.Namespace
	Total      int
	Limit      int
	Offset     int
}

type ListUseCase struct {
	enforcer   listEnforcer
	namespaces nsLister
	counter    listConfigCounter
}

func NewListUseCase(enforcer listEnforcer, namespaces nsLister, counter listConfigCounter) *ListUseCase {
	return &ListUseCase{enforcer: enforcer, namespaces: namespaces, counter: counter}
}

func (uc *ListUseCase) Execute(ctx context.Context, params NSListParams) (*NSListResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	namespaces, err := uc.namespaces.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	namespaces = uc.filterAccessible(claims.Email, namespaces)
	namespaces = filterByQuery(namespaces, params.Query)
	sortNamespaces(namespaces, params.Sort)

	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	total := len(namespaces)
	paginated := paginate(namespaces, params.Offset, limit)

	if err := uc.populateConfigCounts(ctx, paginated); err != nil {
		return nil, err
	}

	return &NSListResult{
		Namespaces: paginated,
		Total:      total,
		Limit:      limit,
		Offset:     params.Offset,
	}, nil
}

// filterAccessible returns only namespaces the caller email can read.
func (uc *ListUseCase) filterAccessible(callerEmail string, namespaces []*domain.Namespace) []*domain.Namespace {
	accessible := namespaces[:0]
	for _, ns := range namespaces {
		allowed, _ := uc.enforcer.Enforce(callerEmail, ns.Name, auth.ObjectNamespace, auth.ActionRead)
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

// paginate returns the slice of namespaces for the given offset and limit.
func paginate(namespaces []*domain.Namespace, offset, limit int) []*domain.Namespace {
	total := len(namespaces)
	if offset >= total {
		return nil
	}

	end := min(offset+limit, total)

	return namespaces[offset:end]
}

// populateConfigCounts sets ConfigCount on each namespace in the slice.
func (uc *ListUseCase) populateConfigCounts(ctx context.Context, namespaces []*domain.Namespace) error {
	for _, ns := range namespaces {
		count, err := uc.counter.CountConfigs(ctx, ns.Name)
		if err != nil {
			return fmt.Errorf("count configs for namespace %q: %w", ns.Name, err)
		}

		ns.ConfigCount = count
	}

	return nil
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
