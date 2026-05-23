package user

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

const defaultListLimit = 20

// ListParams carries pagination, sort, and search options accepted by List.
type ListParams struct {
	Limit  int
	Offset int
	Sort   domain.SortParams
	Query  string
}

// ListResult is the paginated response returned by List.
type ListResult struct {
	Users  []*domain.User
	Total  int
	Limit  int
	Offset int
}

// List returns users the authenticated caller can see, scoped via the groups
// they are allowed to read (EL-4 §7, T5.4).
//
// Flow:
//  1. effective = pdp.EffectiveDomains(caller, "group", "read") — the groups
//     the caller can see, in "group:<name>" subject form.
//  2. If Wildcard: AnyUser=true, no member roll-up.
//  3. Otherwise: PAP.MembersOfScope rolls up group members into a username
//     set; that's the read scope.
//  4. Repo filter applies the set + search; pagination/sort live on the repo.
//  5. Empty effective scope → empty list, not 403 (acceptance: empty
//     responses → empty list).
func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	groupScope := s.pdp.EffectiveDomains(claims.Email, domain.ObjectGroup, domain.ActionRead)

	filter := domain.UserFilter{Search: params.Query}

	switch {
	case groupScope.Wildcard:
		filter.AnyUser = true
	case groupScope.IsEmpty():
		return &ListResult{
			Users:  []*domain.User{},
			Total:  0,
			Limit:  limit,
			Offset: params.Offset,
		}, nil
	default:
		usernames := s.pap.MembersOfScope(groupScope)
		if len(usernames) == 0 {
			return &ListResult{
				Users:  []*domain.User{},
				Total:  0,
				Limit:  limit,
				Offset: params.Offset,
			}, nil
		}

		filter.Usernames = usernames
	}

	repoParams := domain.UserListParams{
		Limit:  limit,
		Offset: params.Offset,
		Sort:   params.Sort,
	}

	users, total, err := s.store.List(ctx, filter, repoParams)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return &ListResult{
		Users:  users,
		Total:  total,
		Limit:  limit,
		Offset: params.Offset,
	}, nil
}
