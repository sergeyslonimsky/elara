package group

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// AddMember adds the given email to the group and syncs Casbin membership records.
//
//nolint:dupl // add and remove share the same structure intentionally
func (s *Service) AddMember(ctx context.Context, groupID, email string) (*domain.Group, error) {
	if err := auth.CheckAccess(ctx, s.enforcer, auth.ObjectAll, "group", auth.ActionWrite); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	group, err := s.store.Get(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	if err = group.AddMember(email); err != nil {
		return nil, fmt.Errorf("add member: %w", err)
	}

	if err = s.store.Update(ctx, group); err != nil {
		return nil, fmt.Errorf(errUpdateGroup, err)
	}

	for _, rule := range s.syncEnforcer.GetRulesForSubject(group.Name) {
		if err := s.syncEnforcer.AddRoleForUser(email, group.Name, rule[2]); err != nil {
			return nil, fmt.Errorf("sync member role: %w", err)
		}
	}

	return group, nil
}

// RemoveMember removes the given email from the group and syncs Casbin membership records.
//
//nolint:dupl // add and remove share the same structure intentionally
func (s *Service) RemoveMember(ctx context.Context, groupID, email string) (*domain.Group, error) {
	if err := auth.CheckAccess(ctx, s.enforcer, auth.ObjectAll, "group", auth.ActionWrite); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	group, err := s.store.Get(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	if err = group.RemoveMember(email); err != nil {
		return nil, fmt.Errorf("remove member: %w", err)
	}

	if err = s.store.Update(ctx, group); err != nil {
		return nil, fmt.Errorf(errUpdateGroup, err)
	}

	for _, rule := range s.syncEnforcer.GetRulesForSubject(group.Name) {
		if err := s.syncEnforcer.RemoveRoleForUser(email, group.Name, rule[2]); err != nil {
			return nil, fmt.Errorf("sync remove member role: %w", err)
		}
	}

	return group, nil
}
