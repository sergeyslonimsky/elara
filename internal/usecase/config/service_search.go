package config

import (
	"context"
	"fmt"
	"sort"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/util/sliceutil"
)

const defaultSearchLimit = 20

func (s *Service) Search(ctx context.Context, params SearchParams) (*SearchResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	scope := s.pdp.EffectiveDomains(claims.Email, domain.ObjectNamespace, domain.ActionRead)
	if scope.IsEmpty() {
		return &SearchResult{
			Results: nil,
			Total:   0,
			Limit:   limit,
			Offset:  params.Offset,
		}, nil
	}

	// Fetch all matching results (bbolt is fast for this).
	results, err := s.storage.SearchByPath(ctx, params.Query, params.Namespace)
	if err != nil {
		return nil, fmt.Errorf("search configs: %w", err)
	}

	// Filter silently by namespace access. Allocate a fresh slice — sharing the
	// backing array of `results` would race with parallel sub-tests where the
	// store mock returns the same slice across calls.
	filtered := make([]*domain.ConfigSummary, 0, len(results))
	for _, r := range results {
		if scope.Contains(r.Namespace) {
			filtered = append(filtered, r)
		}
	}

	results = filtered

	// Sort.
	sortSummaries(results, params.Sort)

	total := len(results)
	offset := params.Offset

	// Paginate.
	paginated := sliceutil.Paginate(results, offset, limit)

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
