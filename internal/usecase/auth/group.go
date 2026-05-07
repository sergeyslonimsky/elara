package auth

//go:generate mockgen -destination=mocks/mock_group.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth groupCreator,groupGetter,groupUpdater,groupDeleter,groupLister,groupEnforcer,groupSyncEnforcer,groupByNameFinder

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

const (
	errGetGroup    = "get group: %w"
	errUpdateGroup = "update group: %w"
)

type groupEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type groupSyncEnforcer interface {
	AddRoleForUser(user, role, domain string) error
	RemoveRoleForUser(user, role, domain string) error
	GetRulesForSubject(subject string) [][]string
}

type groupByNameFinder interface {
	FindByName(ctx context.Context, name string) (*domain.Group, error)
}

type groupCreator interface {
	Create(ctx context.Context, group *domain.Group) error
}

type groupGetter interface {
	Get(ctx context.Context, id string) (*domain.Group, error)
}

type groupUpdater interface {
	Update(ctx context.Context, group *domain.Group) error
}

type groupDeleter interface {
	Delete(ctx context.Context, id string) error
}

type groupLister interface {
	List(ctx context.Context) ([]*domain.Group, error)
}

// CreateGroupUseCase creates a new group.
type CreateGroupUseCase struct {
	enforcer groupEnforcer
	groups   groupCreator
}

// NewCreateGroupUseCase returns a new CreateGroupUseCase.
func NewCreateGroupUseCase(enforcer groupEnforcer, groups groupCreator) *CreateGroupUseCase {
	return &CreateGroupUseCase{enforcer: enforcer, groups: groups}
}

// Execute creates a group with the given name.
func (uc *CreateGroupUseCase) Execute(ctx context.Context, name string) (*domain.Group, error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, "group", auth.ActionWrite); err != nil {
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

	if err := uc.groups.Create(ctx, group); err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}

	return group, nil
}

// GetGroupUseCase returns a group by ID.
type GetGroupUseCase struct {
	enforcer groupEnforcer
	groups   groupGetter
}

// NewGetGroupUseCase returns a new GetGroupUseCase.
func NewGetGroupUseCase(enforcer groupEnforcer, groups groupGetter) *GetGroupUseCase {
	return &GetGroupUseCase{enforcer: enforcer, groups: groups}
}

// Execute returns the group with the given ID.
func (uc *GetGroupUseCase) Execute(ctx context.Context, id string) (*domain.Group, error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, "group", auth.ActionRead); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	group, err := uc.groups.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	return group, nil
}

// UpdateGroupUseCase updates a group's name.
type UpdateGroupUseCase struct {
	enforcer     groupEnforcer
	syncEnforcer groupSyncEnforcer
	groups       interface {
		groupGetter
		groupUpdater
	}
}

// NewUpdateGroupUseCase returns a new UpdateGroupUseCase.
func NewUpdateGroupUseCase(
	enforcer groupEnforcer,
	syncEnforcer groupSyncEnforcer,
	groups interface {
		groupGetter
		groupUpdater
	},
) *UpdateGroupUseCase {
	return &UpdateGroupUseCase{enforcer: enforcer, syncEnforcer: syncEnforcer, groups: groups}
}

// Execute updates the group name and renames Casbin rules if the name changed.
func (uc *UpdateGroupUseCase) Execute(ctx context.Context, id, name string) (*domain.Group, error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, "group", auth.ActionWrite); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	group, err := uc.groups.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	oldName := group.Name
	group.Name = name
	group.UpdatedAt = time.Now().UTC()

	if err = uc.groups.Update(ctx, group); err != nil {
		return nil, fmt.Errorf(errUpdateGroup, err)
	}

	if oldName != name {
		groupRules := uc.syncEnforcer.GetRulesForSubject(oldName)

		// Rename group's own role rules
		for _, rule := range groupRules {
			_ = uc.syncEnforcer.AddRoleForUser(name, rule[1], rule[2])
			_ = uc.syncEnforcer.RemoveRoleForUser(oldName, rule[1], rule[2])
		}

		// Rename members' membership records
		for _, member := range group.Members {
			for _, rule := range groupRules {
				_ = uc.syncEnforcer.AddRoleForUser(member, name, rule[2])
				_ = uc.syncEnforcer.RemoveRoleForUser(member, oldName, rule[2])
			}
		}
	}

	return group, nil
}

// DeleteGroupUseCase deletes a group.
type DeleteGroupUseCase struct {
	enforcer     groupEnforcer
	syncEnforcer groupSyncEnforcer
	groups       interface {
		groupGetter
		groupDeleter
	}
}

// NewDeleteGroupUseCase returns a new DeleteGroupUseCase.
func NewDeleteGroupUseCase(
	enforcer groupEnforcer,
	syncEnforcer groupSyncEnforcer,
	groups interface {
		groupGetter
		groupDeleter
	},
) *DeleteGroupUseCase {
	return &DeleteGroupUseCase{enforcer: enforcer, syncEnforcer: syncEnforcer, groups: groups}
}

// Execute deletes the group and removes all associated Casbin rules.
func (uc *DeleteGroupUseCase) Execute(ctx context.Context, id string) error {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, "group", auth.ActionWrite); err != nil {
		return fmt.Errorf("check access: %w", err)
	}

	group, err := uc.groups.Get(ctx, id)
	if err != nil {
		return fmt.Errorf(errGetGroup, err)
	}

	groupRules := uc.syncEnforcer.GetRulesForSubject(group.Name)

	// Remove per-namespace membership records for all members
	for _, member := range group.Members {
		for _, rule := range groupRules {
			_ = uc.syncEnforcer.RemoveRoleForUser(member, group.Name, rule[2])
		}
	}

	// Remove the group's own role rules
	for _, rule := range groupRules {
		_ = uc.syncEnforcer.RemoveRoleForUser(group.Name, rule[1], rule[2])
	}

	if err := uc.groups.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	return nil
}

// ListGroupsUseCase returns all groups.
type ListGroupsUseCase struct {
	enforcer groupEnforcer
	groups   groupLister
}

// NewListGroupsUseCase returns a new ListGroupsUseCase.
func NewListGroupsUseCase(enforcer groupEnforcer, groups groupLister) *ListGroupsUseCase {
	return &ListGroupsUseCase{enforcer: enforcer, groups: groups}
}

// Execute returns all groups.
func (uc *ListGroupsUseCase) Execute(ctx context.Context) ([]*domain.Group, error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, "group", auth.ActionRead); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	groups, err := uc.groups.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	return groups, nil
}

// AddMemberUseCase adds a member to a group.
type AddMemberUseCase struct {
	enforcer     groupEnforcer
	syncEnforcer groupSyncEnforcer
	groups       interface {
		groupGetter
		groupUpdater
	}
}

// NewAddMemberUseCase returns a new AddMemberUseCase.
func NewAddMemberUseCase(
	enforcer groupEnforcer,
	syncEnforcer groupSyncEnforcer,
	groups interface {
		groupGetter
		groupUpdater
	},
) *AddMemberUseCase {
	return &AddMemberUseCase{enforcer: enforcer, syncEnforcer: syncEnforcer, groups: groups}
}

// Execute adds the given email to the group and syncs Casbin membership records.
//
//nolint:dupl // add and remove share the same structure intentionally
func (uc *AddMemberUseCase) Execute(ctx context.Context, groupID, email string) (*domain.Group, error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, "group", auth.ActionWrite); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	group, err := uc.groups.Get(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	if err = group.AddMember(email); err != nil {
		return nil, fmt.Errorf("add member: %w", err)
	}

	if err = uc.groups.Update(ctx, group); err != nil {
		return nil, fmt.Errorf(errUpdateGroup, err)
	}

	for _, rule := range uc.syncEnforcer.GetRulesForSubject(group.Name) {
		if err := uc.syncEnforcer.AddRoleForUser(email, group.Name, rule[2]); err != nil {
			return nil, fmt.Errorf("sync member role: %w", err)
		}
	}

	return group, nil
}

// RemoveMemberUseCase removes a member from a group.
type RemoveMemberUseCase struct {
	enforcer     groupEnforcer
	syncEnforcer groupSyncEnforcer
	groups       interface {
		groupGetter
		groupUpdater
	}
}

// NewRemoveMemberUseCase returns a new RemoveMemberUseCase.
func NewRemoveMemberUseCase(
	enforcer groupEnforcer,
	syncEnforcer groupSyncEnforcer,
	groups interface {
		groupGetter
		groupUpdater
	},
) *RemoveMemberUseCase {
	return &RemoveMemberUseCase{enforcer: enforcer, syncEnforcer: syncEnforcer, groups: groups}
}

// Execute removes the given email from the group and syncs Casbin membership records.
//
//nolint:dupl // add and remove share the same structure intentionally
func (uc *RemoveMemberUseCase) Execute(ctx context.Context, groupID, email string) (*domain.Group, error) {
	if err := auth.CheckAccess(ctx, uc.enforcer, auth.ObjectAll, "group", auth.ActionWrite); err != nil {
		return nil, fmt.Errorf("check access: %w", err)
	}

	group, err := uc.groups.Get(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	if err = group.RemoveMember(email); err != nil {
		return nil, fmt.Errorf("remove member: %w", err)
	}

	if err = uc.groups.Update(ctx, group); err != nil {
		return nil, fmt.Errorf(errUpdateGroup, err)
	}

	for _, rule := range uc.syncEnforcer.GetRulesForSubject(group.Name) {
		if err := uc.syncEnforcer.RemoveRoleForUser(email, group.Name, rule[2]); err != nil {
			return nil, fmt.Errorf("sync remove member role: %w", err)
		}
	}

	return group, nil
}
