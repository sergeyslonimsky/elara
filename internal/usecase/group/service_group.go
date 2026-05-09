package group

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Create(ctx context.Context, name string) (*domain.Group, error) {
	if err := auth.CheckAccess(ctx, s.enforcer, auth.ObjectAll, "group", auth.ActionWrite); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	now := time.Now().UTC()
	group := &domain.Group{
		ID:        uuid.New().String(),
		Name:      name,
		Members:   []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.Create(ctx, group); err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}

	return group, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Group, error) {
	if err := auth.CheckAccess(ctx, s.enforcer, auth.ObjectAll, "group", auth.ActionRead); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	group, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	return group, nil
}

func (s *Service) Update(ctx context.Context, id, name string) (*domain.Group, error) {
	if err := auth.CheckAccess(ctx, s.enforcer, auth.ObjectAll, "group", auth.ActionWrite); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	group, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	oldName := group.Name
	group.Name = name
	group.UpdatedAt = time.Now().UTC()

	if err = s.store.Update(ctx, group); err != nil {
		return nil, fmt.Errorf(errUpdateGroup, err)
	}

	if oldName != name {
		groupRules := s.syncEnforcer.GetRulesForSubject(oldName)

		// Rename group's own role rules
		for _, rule := range groupRules {
			_ = s.syncEnforcer.AddRoleForUser(name, rule[1], rule[2])
			_ = s.syncEnforcer.RemoveRoleForUser(oldName, rule[1], rule[2])
		}

		// Rename members' membership records
		for _, member := range group.Members {
			for _, rule := range groupRules {
				_ = s.syncEnforcer.AddRoleForUser(member, name, rule[2])
				_ = s.syncEnforcer.RemoveRoleForUser(member, oldName, rule[2])
			}
		}
	}

	return group, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := auth.CheckAccess(ctx, s.enforcer, auth.ObjectAll, "group", auth.ActionWrite); err != nil {
		return fmt.Errorf("check access: %w", err)
	}

	group, err := s.store.Get(ctx, id)
	if err != nil {
		return fmt.Errorf(errGetGroup, err)
	}

	groupRules := s.syncEnforcer.GetRulesForSubject(group.Name)

	// Remove per-namespace membership records for all members
	for _, member := range group.Members {
		for _, rule := range groupRules {
			_ = s.syncEnforcer.RemoveRoleForUser(member, group.Name, rule[2])
		}
	}

	// Remove the group's own role rules
	for _, rule := range groupRules {
		_ = s.syncEnforcer.RemoveRoleForUser(group.Name, rule[1], rule[2])
	}

	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	return nil
}

func (s *Service) List(ctx context.Context) ([]*domain.Group, error) {
	if err := auth.CheckAccess(ctx, s.enforcer, auth.ObjectAll, "group", auth.ActionRead); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	groups, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	return groups, nil
}
