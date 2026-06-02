package group

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// GetResult bundles the group entity with its members visible to the
// caller (filtered per derived User:Read) and full permission set
// (visibility derives from holding Group:Read on the group itself — no
// per-permission filter).
type GetResult struct {
	Group          *domain.Group
	VisibleMembers []string
	Permissions    []domain.Permission
}

func (s *Service) Get(ctx context.Context, actor domain.AuthInfo, name string) (*GetResult, error) {
	group, err := s.store.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	return &GetResult{
		Group:          group,
		VisibleMembers: s.filterVisibleMembers(ctx, actor, s.pap.GroupMembers(group.Name)),
		Permissions:    s.pap.GroupPermissions(group.Name),
	}, nil
}
