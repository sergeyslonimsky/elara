package config

import (
	"context"
	"fmt"
	"sort"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

const defaultSearchLimit = 20

type searchEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type configSearcher interface {
	SearchByPath(ctx context.Context, query, namespace string) ([]*domain.ConfigSummary, error)
}

type SearchParams struct {
	Query     string
	Namespace string
	Limit     int
	Offset    int
	Sort      domain.SortParams
}

type SearchResult struct {
	Results []*domain.ConfigSummary
	Total   int
	Limit   int
	Offset  int
}

type SearchUseCase struct {
	enforcer searchEnforcer
	configs  configSearcher
}

func NewSearchUseCase(enforcer searchEnforcer, configs configSearcher) *SearchUseCase {
	return &SearchUseCase{enforcer: enforcer, configs: configs}
}

func (uc *SearchUseCase) Execute(ctx context.Context, params SearchParams) (*SearchResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	// Fetch all matching results (bbolt is fast for this).
	results, err := uc.configs.SearchByPath(ctx, params.Query, params.Namespace)
	if err != nil {
		return nil, fmt.Errorf("search configs: %w", err)
	}

	// Filter silently by namespace access.
	filtered := results[:0]
	for _, r := range results {
		allowed, _ := uc.enforcer.Enforce(claims.Email, r.Namespace, "config", "read")
		if allowed {
			filtered = append(filtered, r)
		}
	}

	results = filtered

	// Sort.
	sortSummaries(results, params.Sort)

	total := len(results)
	offset := params.Offset

	// Paginate.
	var paginated []*domain.ConfigSummary
	if offset < total {
		end := min(offset+limit, total)
		paginated = results[offset:end]
	}

	return &SearchResult{
		Results: paginated,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

func sortSummaries(summaries []*domain.ConfigSummary, params domain.SortParams) {
	sort.Slice(summaries, func(i, j int) bool {
		a, b := summaries[i], summaries[j]

		var less bool

		switch params.Field {
		case "modified":
			less = a.UpdatedAt.Before(b.UpdatedAt)
		default: // "name" or empty
			less = a.Path < b.Path
		}

		if params.Desc {
			return !less
		}

		return less
	})
}
