package filter

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Users returns the users selectable by the caller.
//
// Unlike namespaces and groups, users have no per-resource scope: User
// permissions are global (domain "*"). The caller therefore either sees every
// user — when they hold a global User action satisfying the request — or none.
// Group-derived visibility is intentionally NOT used: a freshly created,
// group-less user must remain selectable so the caller can, e.g., add them to
// a group. Every returned item carries the same global User action set.
func (s *Service) Users(ctx context.Context, actor domain.AuthInfo, query Query) ([]Item, error) {
	perms, err := s.perms.ListPermissions(actor.Email)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	global, _ := scopedActions(perms, domain.ObjectUser)
	if !grants(global, query.Actions) {
		return []Item{}, nil
	}

	users, _, err := s.users.List(
		ctx,
		domain.UserFilter{AnyUser: true, Search: query.Search},
		domain.UserListParams{},
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	actions := resolve(global)

	items := make([]Item, 0, len(users))
	for _, u := range users {
		value := u.Name
		if value == "" {
			value = u.Email
		}

		items = append(items, Item{Key: u.Email, Value: value, Actions: actions})
	}

	return items, nil
}
