package auth

//go:generate mockgen -destination=mocks/mock_group.go -package=auth_mock github.com/sergeyslonimsky/elara/internal/usecase/auth groupCreator,groupGetter,groupUpdater,groupDeleter,groupLister

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const (
	errGetGroup    = "get group: %w"
	errUpdateGroup = "update group: %w"
)

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
	groups groupCreator
}

// NewCreateGroupUseCase returns a new CreateGroupUseCase.
func NewCreateGroupUseCase(groups groupCreator) *CreateGroupUseCase {
	return &CreateGroupUseCase{groups: groups}
}

// Execute creates a group with the given name.
func (uc *CreateGroupUseCase) Execute(ctx context.Context, name string) (*domain.Group, error) {
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
	groups groupGetter
}

// NewGetGroupUseCase returns a new GetGroupUseCase.
func NewGetGroupUseCase(groups groupGetter) *GetGroupUseCase {
	return &GetGroupUseCase{groups: groups}
}

// Execute returns the group with the given ID.
func (uc *GetGroupUseCase) Execute(ctx context.Context, id string) (*domain.Group, error) {
	group, err := uc.groups.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	return group, nil
}

// UpdateGroupUseCase updates a group's name.
type UpdateGroupUseCase struct {
	groups interface {
		groupGetter
		groupUpdater
	}
}

// NewUpdateGroupUseCase returns a new UpdateGroupUseCase.
func NewUpdateGroupUseCase(groups interface {
	groupGetter
	groupUpdater
},
) *UpdateGroupUseCase {
	return &UpdateGroupUseCase{groups: groups}
}

// Execute updates the group name.
func (uc *UpdateGroupUseCase) Execute(ctx context.Context, id, name string) (*domain.Group, error) {
	group, err := uc.groups.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	group.Name = name
	group.UpdatedAt = time.Now().UTC()

	if err = uc.groups.Update(ctx, group); err != nil {
		return nil, fmt.Errorf(errUpdateGroup, err)
	}

	return group, nil
}

// DeleteGroupUseCase deletes a group.
type DeleteGroupUseCase struct {
	groups groupDeleter
}

// NewDeleteGroupUseCase returns a new DeleteGroupUseCase.
func NewDeleteGroupUseCase(groups groupDeleter) *DeleteGroupUseCase {
	return &DeleteGroupUseCase{groups: groups}
}

// Execute deletes the group with the given ID.
func (uc *DeleteGroupUseCase) Execute(ctx context.Context, id string) error {
	if err := uc.groups.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	return nil
}

// ListGroupsUseCase returns all groups.
type ListGroupsUseCase struct {
	groups groupLister
}

// NewListGroupsUseCase returns a new ListGroupsUseCase.
func NewListGroupsUseCase(groups groupLister) *ListGroupsUseCase {
	return &ListGroupsUseCase{groups: groups}
}

// Execute returns all groups.
func (uc *ListGroupsUseCase) Execute(ctx context.Context) ([]*domain.Group, error) {
	groups, err := uc.groups.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	return groups, nil
}

// AddMemberUseCase adds a member to a group.
type AddMemberUseCase struct {
	groups interface {
		groupGetter
		groupUpdater
	}
}

// NewAddMemberUseCase returns a new AddMemberUseCase.
func NewAddMemberUseCase(groups interface {
	groupGetter
	groupUpdater
},
) *AddMemberUseCase {
	return &AddMemberUseCase{groups: groups}
}

// Execute adds the given email to the group.
func (uc *AddMemberUseCase) Execute(ctx context.Context, groupID, email string) (*domain.Group, error) {
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

	return group, nil
}

// RemoveMemberUseCase removes a member from a group.
type RemoveMemberUseCase struct {
	groups interface {
		groupGetter
		groupUpdater
	}
}

// NewRemoveMemberUseCase returns a new RemoveMemberUseCase.
func NewRemoveMemberUseCase(groups interface {
	groupGetter
	groupUpdater
},
) *RemoveMemberUseCase {
	return &RemoveMemberUseCase{groups: groups}
}

// Execute removes the given email from the group.
func (uc *RemoveMemberUseCase) Execute(ctx context.Context, groupID, email string) (*domain.Group, error) {
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

	return group, nil
}
