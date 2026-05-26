package group

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	"github.com/sergeyslonimsky/elara/internal/util/sliceutil"
)

// UpdateData carries the metadata-only fields the group entity owns in
// bbolt. Members and permissions are managed by UpdateMembers and
// UpdatePermissions respectively.
type UpdateData struct {
	ID                      string
	Name                    string
	Description             string
	ExpectedMetadataVersion *int64
}

// Update mutates a group's metadata (name, description). MetadataVersion
// is bumped and the Casbin subject is renamed in the same transaction when
// the name changes.
func (s *Service) Update(
	ctx context.Context,
	_ domain.AuthInfo,
	data UpdateData,
) (*domain.Group, error) {
	var updated *domain.Group

	err := s.pap.Write(ctx, func(tx storage.Tx, w *authz.PAPTx) error {
		existing, err := s.loadMutableGroup(ctx, tx, data.ID)
		if err != nil {
			return err
		}
		if err := domain.CheckVersion(data.ExpectedMetadataVersion, existing.MetadataVersion); err != nil {
			return err
		}

		oldName := existing.Name
		if data.Name != oldName {
			// Names are not the primary key but every Casbin subject and
			// every PAP.UserGroupNames consumer keys off them. Two groups
			// with the same name break FindByName-based lookups silently.
			switch other, err := s.store.WithTx(tx).FindByName(ctx, data.Name); {
			case err == nil && other != nil && other.ID != existing.ID:
				return domain.NewAlreadyExistsError("group", data.Name)
			case err != nil && !errors.Is(err, domain.ErrNotFound):
				return fmt.Errorf("check name uniqueness: %w", err)
			}
		}
		existing.Name = data.Name
		existing.Description = data.Description
		existing.MetadataVersion++
		existing.UpdatedAt = time.Now().UTC()

		if err := s.store.WithTx(tx).Update(ctx, existing); err != nil {
			return fmt.Errorf(errUpdateGroup, err)
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

	return updated, nil
}

// UpdateMembersData carries the explicit add/remove delta for membership.
type UpdateMembersData struct {
	GroupID                string
	AddEmails              []string
	RemoveEmails           []string
	ExpectedMembersVersion *int64
}

// UpdateMembersResult bundles the updated group with the member list
// visible to the caller (filtered per derived User:Read).
type UpdateMembersResult struct {
	Group          *domain.Group
	VisibleMembers []string
}

// UpdateMembers applies an explicit membership delta. Add of an existing
// member is a no-op; remove of an absent member is a no-op. An email
// appearing in both add_emails and remove_emails returns InvalidArgument.
//
// Anti-escalation: each effective added email receives the group's full
// permission set, so the actor must hold every one of those permissions.
// Removals narrow and skip the escalation check.
func (s *Service) UpdateMembers(
	ctx context.Context,
	actor domain.AuthInfo,
	data UpdateMembersData,
) (*UpdateMembersResult, error) {
	if v, dup := sliceutil.FirstOverlap(data.AddEmails, data.RemoveEmails); dup {
		return nil, domain.NewValidationError("email", fmt.Sprintf("%q appears in both add and remove", v))
	}

	var result *UpdateMembersResult

	err := s.pap.Write(ctx, func(tx storage.Tx, w *authz.PAPTx) error {
		existing, err := s.loadMutableGroup(ctx, tx, data.GroupID)
		if err != nil {
			return err
		}
		if err := domain.CheckVersion(data.ExpectedMembersVersion, existing.MembersVersion); err != nil {
			return err
		}

		current := s.pap.GroupMembers(existing.Name)
		currentSet := sliceutil.ToSet(current)
		added := sliceutil.NotIn(data.AddEmails, currentSet)
		removed := sliceutil.In(data.RemoveEmails, currentSet)

		if len(added) > 0 {
			if err := s.scope.RequireMembershipGrant(actor.Email, existing.Name); err != nil {
				return err
			}
		}

		if len(added) == 0 && len(removed) == 0 {
			result = &UpdateMembersResult{
				Group:          existing,
				VisibleMembers: s.filterVisibleMembers(ctx, actor, current),
			}

			return nil
		}

		if err := w.ApplyMemberDeltas(existing.Name, added, removed); err != nil {
			return fmt.Errorf("apply member deltas: %w", err)
		}

		existing.MembersVersion++
		existing.UpdatedAt = time.Now().UTC()
		if err := s.store.WithTx(tx).Update(ctx, existing); err != nil {
			return fmt.Errorf(errUpdateGroup, err)
		}

		post := sliceutil.ComposePost(current, added, removed)
		result = &UpdateMembersResult{Group: existing, VisibleMembers: s.filterVisibleMembers(ctx, actor, post)}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update group members: %w", err)
	}

	return result, nil
}

// UpdatePermissionsData carries the explicit add/remove delta for the
// group's permission set.
type UpdatePermissionsData struct {
	GroupID                    string
	Add                        []domain.Permission
	Remove                     []domain.Permission
	ExpectedPermissionsVersion *int64
}

// UpdatePermissionsResult bundles the updated group with its current
// permission set.
type UpdatePermissionsResult struct {
	Group       *domain.Group
	Permissions []domain.Permission
}

// UpdatePermissions applies an explicit permission delta. Add of an
// existing permission is a no-op; remove of an absent permission is a
// no-op. A permission in both add and remove returns InvalidArgument.
//
// Anti-escalation:
//   - per-delta: actor must hold each effectively-added permission.
//   - cascade: if the group has members and any delta applies, actor
//     must hold every permission the group will hold post-update —
//     because the change cascades to existing members through their
//     membership g-rules.
func (s *Service) UpdatePermissions(
	ctx context.Context,
	actor domain.AuthInfo,
	data UpdatePermissionsData,
) (*UpdatePermissionsResult, error) {
	if p, dup := sliceutil.FirstOverlap(data.Add, data.Remove); dup {
		return nil, domain.NewValidationError(
			"permissions",
			fmt.Sprintf("%s:%s on %s appears in both add and remove", p.Object, p.Action, p.Domain),
		)
	}

	var result *UpdatePermissionsResult

	err := s.pap.Write(ctx, func(tx storage.Tx, w *authz.PAPTx) error {
		existing, err := s.loadMutableGroup(ctx, tx, data.GroupID)
		if err != nil {
			return err
		}
		if err := domain.CheckVersion(data.ExpectedPermissionsVersion, existing.PermissionsVersion); err != nil {
			return err
		}

		current, err := w.GroupPermissions(existing.Name)
		if err != nil {
			return fmt.Errorf("pap group permissions: %w", err)
		}
		currentSet := sliceutil.ToSet(current)
		added := sliceutil.NotIn(data.Add, currentSet)
		removed := sliceutil.In(data.Remove, currentSet)

		if err := s.boundaryCheckPerms(actor, added); err != nil {
			return err
		}

		if len(added) == 0 && len(removed) == 0 {
			result = &UpdatePermissionsResult{Group: existing, Permissions: current}

			return nil
		}

		post := sliceutil.ComposePost(current, added, removed)
		if err := s.cascadeCheckPerms(actor, existing.Name, post); err != nil {
			return err
		}

		if err := w.ApplyPermissionDeltas(existing.Name, added, removed); err != nil {
			return fmt.Errorf("apply permission deltas: %w", err)
		}

		existing.PermissionsVersion++
		existing.UpdatedAt = time.Now().UTC()
		if err := s.store.WithTx(tx).Update(ctx, existing); err != nil {
			return fmt.Errorf(errUpdateGroup, err)
		}

		result = &UpdatePermissionsResult{Group: existing, Permissions: post}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update group permissions: %w", err)
	}

	return result, nil
}

// boundaryCheckPerms asserts each effectively-added permission lies within
// the actor's own permission set.
func (s *Service) boundaryCheckPerms(actor domain.AuthInfo, added []domain.Permission) error {
	for _, p := range added {
		if !s.pdp.Has(actor.Email, p) {
			return domain.ErrPermissionEscalation
		}
	}

	return nil
}

// cascadeCheckPerms asserts that if the group has members, the actor holds
// every permission the group will hold post-update — because the change
// cascades to existing members through their membership g-rules. Called
// only when an effective delta exists (no-op deltas skip this check).
func (s *Service) cascadeCheckPerms(actor domain.AuthInfo, groupName string, post []domain.Permission) error {
	if len(s.pap.GroupMembers(groupName)) == 0 {
		return nil
	}
	for _, p := range post {
		if !s.pdp.Has(actor.Email, p) {
			return domain.ErrPermissionEscalation
		}
	}

	return nil
}
