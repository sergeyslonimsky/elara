package user

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
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

// List returns users visible to the caller using a two-tier authorization
// model:
//
//  1. User:Read * (global) — full unfiltered list (subject to search/pagination).
//  2. Otherwise — derived through Group:Read: only users who belong to at
//     least one group the caller can read are returned. Unassigned users
//     are invisible in this mode.
//
// An empty result (neither User:Read * nor any Group:Read scope) is not
// an error; pagination returns an empty page.
func (s *Service) List(ctx context.Context, actor domain.AuthInfo, params ListParams) (*ListResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	filter := domain.UserFilter{Search: params.Query}

	// Fast path: global User:Read.
	if s.pdp.HasUserReadGlobal(actor.Email) {
		filter.AnyUser = true
	} else {
		groupScope := s.pdp.EffectiveDomains(actor.Email, domain.ObjectGroup, domain.ActionRead)
		switch {
		case groupScope.Wildcard:
			filter.AnyUser = true
		case groupScope.IsEmpty():
			return emptyList(limit, params.Offset), nil
		default:
			usernames := s.pap.MembersOfScope(groupScope)
			if len(usernames) == 0 {
				return emptyList(limit, params.Offset), nil
			}
			filter.Usernames = usernames
		}
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

func emptyList(limit, offset int) *ListResult {
	return &ListResult{
		Users:  []*domain.User{},
		Total:  0,
		Limit:  limit,
		Offset: offset,
	}
}
