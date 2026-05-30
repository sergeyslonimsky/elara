package filter

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Groups returns the groups the caller can act on with at least one of the
// requested actions. Each item carries the caller's full action set on that
// group.
//
// Group permission domains are keyed by group ID ("group:<id>"), which the
// repository's name-keyed GroupFilter cannot express. We therefore enumerate
// every group (the repo applies the search) and resolve each against the scope
// by ID in memory; only groups the caller can act on are returned, so the full
// enumeration never leaks inaccessible groups.
func (s *Service) Groups(ctx context.Context, actor domain.AuthInfo, query Query) ([]Item, error) {
	perms, err := s.perms.ListPermissions(actor.Email)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	wildcard, explicit := scopedActions(perms, domain.ObjectGroup)
	if wildcard.empty() && len(explicit) == 0 {
		return []Item{}, nil
	}

	groups, _, err := s.groups.List(
		ctx,
		domain.GroupFilter{Wildcard: true, Search: query.Search},
		domain.GroupListParams{},
	)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	items := make([]Item, 0, len(groups))
	for _, g := range groups {
		have := actionSet{}
		have.union(wildcard)

		if set, ok := explicit[domain.GroupResource(g.ID)]; ok {
			have.union(set)
		}

		if !grants(have, query.Actions) {
			continue
		}

		items = append(items, Item{Key: g.ID, Value: g.Name, Actions: resolve(have)})
	}

	return items, nil
}
