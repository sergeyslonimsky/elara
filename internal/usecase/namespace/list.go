package namespace

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const defaultListLimit = 20

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
	namespaces nsLister
	counter    listConfigCounter
}

func NewListUseCase(namespaces nsLister, counter listConfigCounter) *ListUseCase {
	return &ListUseCase{namespaces: namespaces, counter: counter}
}

func (uc *ListUseCase) Execute(ctx context.Context, params NSListParams) (*NSListResult, error) {
	namespaces, err := uc.namespaces.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	// Filter by query if provided.
	if params.Query != "" {
		queryLower := strings.ToLower(params.Query)

		filtered := make([]*domain.Namespace, 0, len(namespaces))
		for _, ns := range namespaces {
			if strings.Contains(strings.ToLower(ns.Name), queryLower) {
				filtered = append(filtered, ns)
			}
		}

		namespaces = filtered
	}

	// Sort before pagination (config count not needed for sorting).
	sortNamespaces(namespaces, params.Sort)

	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	total := len(namespaces)
	offset := params.Offset

	var paginated []*domain.Namespace
	if offset < total {
		end := min(offset+limit, total)
		paginated = namespaces[offset:end]
	}

	// Count configs only for the paginated page (avoids N+1 for all namespaces).
	for _, ns := range paginated {
		count, err := uc.counter.CountConfigs(ctx, ns.Name)
		if err != nil {
			return nil, fmt.Errorf("count configs for namespace %q: %w", ns.Name, err)
		}

		ns.ConfigCount = count
	}

	return &NSListResult{
		Namespaces: paginated,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
	}, nil
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
