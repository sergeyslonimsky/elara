package group

import (
	"context"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	"github.com/sergeyslonimsky/elara/internal/util/sliceutil"
)

// UpdateData carries the metadata-only fields the group entity owns in
// bbolt. Members and Permissions are managed by UpdateMembers and
// UpdatePermissions respectively — Casbin is the source of truth for both,
// so mixing them into this call would re-introduce the dual-write drift the
// split was meant to eliminate.
type UpdateData struct {
	ID          string
	Name        string
	Description string
	Version     int64
}

// Update mutates a group's metadata (name, description) under optimistic
// concurrency control. Membership and permissions are intentionally not
// accepted here.
func (s *Service) Update(
	ctx context.Context,
	_ domain.AuthInfo,
	data UpdateData,
) (*domain.Group, error) {
	var updated *domain.Group

	err := s.pap.Write(ctx, func(tx storage.Tx, w *authz.PAPTx) error {
		existing, err := s.loadGroupForUpdate(ctx, tx, data.ID, data.Version)
		if err != nil {
			return err
		}

		oldName := existing.Name
		if err := s.applyEntityUpdate(ctx, tx, existing, data); err != nil {
			return err
		}
		updated = existing

		if err := w.RenameGroup(oldName, data.Name); err != nil {
			return fmt.Errorf("rename group: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update group: %w", err)
	}

	updated.Members = s.pap.GroupMembers(updated.Name)

	return updated, nil
}

func (s *Service) applyEntityUpdate(
	ctx context.Context,
	tx storage.Tx,
	existing *domain.Group,
	data UpdateData,
) error {
	existing.Name = data.Name
	existing.Description = data.Description
	existing.Version++
	existing.UpdatedAt = time.Now().UTC()

	if err := s.store.WithTx(tx).Update(ctx, existing); err != nil {
		return fmt.Errorf(errUpdateGroup, err)
	}

	return nil
}

// UpdateMembersData carries the canonical desired member set for a group.
// Members is the full set; the service diffs against the current Casbin
// state and only operates on the symmetric difference.
type UpdateMembersData struct {
	GroupID string
	Members []string
}

// UpdateMembers replaces the group's membership g-rules with the given set.
//
// Authorization (anti-escalation): adding a member is equivalent to
// granting them every permission the group currently holds, so the actor
// must hold every one of those permissions. Removal narrows and requires
// no escalation check.
func (s *Service) UpdateMembers(
	ctx context.Context,
	user domain.AuthInfo,
	data UpdateMembersData,
) (*domain.Group, error) {
	var updated *domain.Group

	err := s.pap.Write(ctx, func(tx storage.Tx, w *authz.PAPTx) error {
		existing, err := s.loadMutableGroup(ctx, tx, data.GroupID)
		if err != nil {
			return err
		}

		current := s.pap.GroupMembers(existing.Name)
		added, removed := sliceutil.Diff(current, data.Members)

		if len(added) > 0 {
			groupPerms, err := w.GroupPermissions(existing.Name)
			if err != nil {
				return fmt.Errorf("pap group permissions: %w", err)
			}
			for _, p := range groupPerms {
				if !s.pdp.Has(user.Email, p) {
					return domain.ErrPermissionEscalation
				}
			}
		}

		if err := w.ApplyMemberDeltas(existing.Name, added, removed); err != nil {
			return fmt.Errorf("apply member deltas: %w", err)
		}

		updated = existing

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update group members: %w", err)
	}

	updated.Members = s.pap.GroupMembers(updated.Name)

	return updated, nil
}

// UpdatePermissionsData carries the canonical desired permission set for a
// group. The service diffs against current Casbin policy and operates only
// on the symmetric difference.
type UpdatePermissionsData struct {
	GroupID     string
	Permissions []domain.Permission
}

// UpdatePermissions replaces the group's permission p-rules with the given
// set.
//
// Authorization: each added or removed permission must lie within the
// actor's own boundary (delta check). Additionally, if the group has
// members, the actor must hold every permission the group will hold
// post-update — because the change cascades to existing members.
func (s *Service) UpdatePermissions(
	ctx context.Context,
	user domain.AuthInfo,
	data UpdatePermissionsData,
) (*domain.Group, error) {
	var updated *domain.Group

	err := s.pap.Write(ctx, func(tx storage.Tx, w *authz.PAPTx) error {
		existing, err := s.loadMutableGroup(ctx, tx, data.GroupID)
		if err != nil {
			return err
		}

		oldPerms, err := w.GroupPermissions(existing.Name)
		if err != nil {
			return fmt.Errorf("pap group permissions: %w", err)
		}

		pAdded, pRemoved := sliceutil.Diff(oldPerms, data.Permissions)

		for _, p := range pAdded {
			if !s.pdp.Has(user.Email, p) {
				return domain.ErrPermissionEscalation
			}
		}
		for _, p := range pRemoved {
			if !s.pdp.Has(user.Email, p) {
				return domain.ErrPermissionEscalation
			}
		}

		members := s.pap.GroupMembers(existing.Name)
		if len(members) > 0 {
			for _, p := range data.Permissions {
				if !s.pdp.Has(user.Email, p) {
					return domain.ErrPermissionEscalation
				}
			}
		}

		if err := w.ApplyPermissionDeltas(existing.Name, pAdded, pRemoved); err != nil {
			return fmt.Errorf("apply permission deltas: %w", err)
		}

		updated = existing

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update group permissions: %w", err)
	}

	updated.Members = s.pap.GroupMembers(updated.Name)

	return updated, nil
}
