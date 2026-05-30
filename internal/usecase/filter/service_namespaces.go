package filter

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Namespaces returns the namespaces the caller can act on with at least one of
// the requested actions. Each item carries the caller's full action set on
// that namespace. Namespace permission domains are the namespace names, so the
// explicit scope maps 1:1 onto NamespaceFilter.Names; a wildcard grant lists
// every namespace. An empty scope yields an empty list, never an error.
func (s *Service) Namespaces(
	ctx context.Context,
	actor domain.AuthInfo,
	query Query,
) ([]Item, error) {
	perms, err := s.perms.ListPermissions(actor.Email)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	wildcard, explicit := scopedActions(perms, domain.ObjectNamespace)
	if wildcard.empty() && len(explicit) == 0 {
		return []Item{}, nil
	}

	filter := domain.NamespaceFilter{Search: query.Search}
	if wildcard.empty() {
		filter.Names = make(map[string]struct{}, len(explicit))
		for name := range explicit {
			filter.Names[name] = struct{}{}
		}
	} else {
		filter.Wildcard = true
	}

	namespaces, _, err := s.namespaces.List(ctx, filter, domain.NamespaceListParams{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	items := make([]Item, 0, len(namespaces))
	for _, ns := range namespaces {
		have := actionSet{}
		have.union(wildcard)

		if set, ok := explicit[ns.Name]; ok {
			have.union(set)
		}

		if !grants(have, query.Actions) {
			continue
		}

		items = append(items, Item{Key: ns.Name, Value: ns.Name, Actions: resolve(have)})
	}

	return items, nil
}
