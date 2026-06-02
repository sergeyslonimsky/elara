package group

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

// CreateData carries the parameters CreateGroup accepts.
//
// InitialMembers, InitialPermissions, and InitialManagerGroupIDs are applied
// atomically alongside the group's creation. Coarse authorization
// (Group:Create *) is enforced in the handler; anti-escalation is enforced
// here, inside the same PAP write transaction as the underlying writes.
type CreateData struct {
	Name                     string
	Description              string
	InitialMembers           []string
	InitialPermissions       []domain.Permission
	InitialManagerGroupNames []string
}

// CreateResult bundles the created group with its initial members
// (filtered to those visible to the caller) and permissions, so the
// handler can render the response without re-fetching.
type CreateResult struct {
	Group          *domain.Group
	VisibleMembers []string
	Permissions    []domain.Permission
}

// Create creates a new group with optional initial state. The orchestration
// is intentionally thin — each step is a private helper so the read order
// matches the proto contract:
//
//  1. authorizeCreate          — actor boundary on perms; Group:Write on
//     each manager group; cascade check that each manager group already
//     dominates initial_permissions (without this its members would receive
//     Group:Write on the new group but anti-escalation would block their
//     UpdateGroupMembers — a confusing half-state).
//  2. persistEntity            — uuid + metadata + name uniqueness, bbolt write.
//  3. applyInitialPermissions  — p-rules on the new subject.
//  4. applyInitialMembers      — g-rules on the new subject.
//  5. wireInitialManagers      — Group:Write group:<new-id> on each manager.
func (s *Service) Create(
	ctx context.Context,
	actor domain.AuthInfo,
	data CreateData,
) (*CreateResult, error) {
	var result *CreateResult

	err := s.pap.Write(ctx, func(ctx context.Context, w *authz.PAPTx) error {
		managerGroups, err := s.authorizeCreate(ctx, actor, data)
		if err != nil {
			return err
		}

		group, err := s.persistEntity(ctx, data)
		if err != nil {
			return err
		}

		if err := s.applyInitialPermissions(w, group, data.InitialPermissions); err != nil {
			return err
		}
		if err := s.applyInitialMembers(w, group, data.InitialMembers); err != nil {
			return err
		}
		if err := s.wireInitialManagers(w, group, managerGroups); err != nil {
			return err
		}

		// Persist any bumped version counters from the steps above.
		if group.PermissionsVersion > 0 || group.MembersVersion > 0 {
			if err := s.store.Update(ctx, group); err != nil {
				return fmt.Errorf("persist initial versions: %w", err)
			}
		}

		result = &CreateResult{
			Group:          group,
			VisibleMembers: s.filterVisibleMembers(ctx, actor, data.InitialMembers),
			Permissions:    data.InitialPermissions,
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create group transaction: %w", err)
	}

	return result, nil
}

// authorizeCreate runs the anti-escalation checks that must succeed before
// any persistence happens. Returns the manager groups loaded by id so the
// later wireInitialManagers step does not re-fetch.
func (s *Service) authorizeCreate(
	ctx context.Context,
	actor domain.AuthInfo,
	data CreateData,
) ([]*domain.Group, error) {
	for _, p := range data.InitialPermissions {
		if !s.pdp.Has(actor.UserID, p) {
			return nil, domain.ErrPermissionEscalation
		}
	}

	managerGroups := make([]*domain.Group, 0, len(data.InitialManagerGroupNames))
	for _, name := range data.InitialManagerGroupNames {
		if !s.pdp.HasGroup(actor.UserID, name, domain.ActionWrite) {
			return nil, domain.ErrForbidden
		}
		mg, err := s.store.Get(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("get manager group %s: %w", name, err)
		}
		// Cascade: the manager group itself must dominate every initial
		// permission. Without this, members of the manager group would
		// receive Group:Write on the new group but anti-escalation on
		// UpdateGroupMembers would prevent them from actually adding
		// members — a confusing half-state. PDP.HasForGroup queries the
		// group subject as principal so Casbin resolves through any
		// roles it holds.
		for _, p := range data.InitialPermissions {
			if !s.pdp.HasForGroup(mg.Name, p) {
				return nil, fmt.Errorf(
					"manager group %s does not dominate %s:%s on %s: %w",
					mg.Name, p.Object, p.Action, p.Domain, domain.ErrPermissionEscalation,
				)
			}
		}
		managerGroups = append(managerGroups, mg)
	}

	return managerGroups, nil
}

// persistEntity allocates the group's id, validates, asserts name uniqueness,
// and writes it to bbolt. MetadataVersion=1 reflects the create event.
func (s *Service) persistEntity(
	ctx context.Context,
	data CreateData,
) (*domain.Group, error) {
	// Group names must be unique — every Casbin subject and every
	// Get-based lookup keys off them. bbolt is keyed by ID, so the
	// repo's own collision check doesn't catch duplicate names.
	switch existing, err := s.store.Get(ctx, data.Name); {
	case err == nil && existing != nil:
		return nil, fmt.Errorf(
			"check name uniqueness: %w",
			domain.NewAlreadyExistsError("group", data.Name),
		)
	case err != nil && !errors.Is(err, domain.ErrNotFound):
		return nil, fmt.Errorf("check name uniqueness: %w", err)
	}

	now := time.Now().UTC()
	group := &domain.Group{
		Name:            data.Name,
		Description:     data.Description,
		MetadataVersion: 1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := group.Validate(); err != nil {
		return nil, fmt.Errorf("validate group: %w", err)
	}
	if err := s.store.Create(ctx, group); err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}

	return group, nil
}

func (s *Service) applyInitialPermissions(
	w *authz.PAPTx,
	group *domain.Group,
	perms []domain.Permission,
) error {
	if len(perms) == 0 {
		return nil
	}
	if err := w.ApplyPermissionDeltas(group.Name, perms, nil); err != nil {
		return fmt.Errorf("apply initial permissions: %w", err)
	}
	group.PermissionsVersion = 1

	return nil
}

func (s *Service) applyInitialMembers(w *authz.PAPTx, group *domain.Group, members []string) error {
	if len(members) == 0 {
		return nil
	}
	if err := w.ApplyMemberDeltas(group.Name, members, nil); err != nil {
		return fmt.Errorf("apply initial members: %w", err)
	}
	group.MembersVersion = 1

	return nil
}

// wireInitialManagers grants `Group:Write group:<new-id>` to each manager
// group, so its members can manage the new group post-creation.
func (s *Service) wireInitialManagers(
	w *authz.PAPTx,
	group *domain.Group,
	managers []*domain.Group,
) error {
	if len(managers) == 0 {
		return nil
	}
	grant := []domain.Permission{{
		Object: domain.ObjectGroup,
		Action: domain.ActionWrite,
		Domain: domain.GroupResource(group.Name),
	}}
	for _, mg := range managers {
		if err := w.ApplyPermissionDeltas(mg.Name, grant, nil); err != nil {
			return fmt.Errorf("grant manager %s: %w", mg.Name, err)
		}
	}

	return nil
}
