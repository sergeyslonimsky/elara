package group

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// ListParams carries pagination, sort, and search options accepted by List.
type ListParams struct {
	Limit  int
	Offset int
	Sort   domain.SortParams
	Query  string
}

// ListResult is the paginated response returned by List.
type ListResult struct {
	Groups []*domain.Group
	Total  int
	Limit  int
	Offset int
}

// List returns groups the authenticated caller can read.
//
// Filtering happens at the repository: the caller's effective group set
// (from PDP.EffectiveDomains for object=group action=read) is translated
// into a GroupFilter — no post-fetch pdp.Has loop. An empty effective set
// returns an empty list, not an error (EL-4 §7 acceptance: empty
// responses → empty list, not 403).
func (s *Service) List(ctx context.Context, user domain.AuthInfo, params ListParams) (*ListResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	scope := s.pdp.EffectiveDomains(user.Email, domain.ObjectGroup, domain.ActionRead)
	if scope.IsEmpty() {
		return &ListResult{
			Groups: []*domain.Group{},
			Total:  0,
			Limit:  limit,
			Offset: params.Offset,
		}, nil
	}

	filter := domain.GroupFilter{
		Names:    s.pap.GroupNamesFromScope(scope),
		Wildcard: scope.Wildcard,
		Search:   params.Query,
	}
	repoParams := domain.GroupListParams{
		Limit:  limit,
		Offset: params.Offset,
		Sort:   params.Sort,
	}

	groups, total, err := s.store.List(ctx, filter, repoParams)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	for _, g := range groups {
		g.Members = s.pap.GroupMembers(g.Name)
	}

	return &ListResult{
		Groups: groups,
		Total:  total,
		Limit:  limit,
		Offset: params.Offset,
	}, nil
}
