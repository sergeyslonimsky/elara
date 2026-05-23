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

type UpdateData struct {
	ID          string
	Name        string
	Description string
	Permissions []domain.Permission
	Members     []string
	Version     int64
}

func (s *Service) Update(
	ctx context.Context,
	user domain.AuthInfo,
	data UpdateData,
) (*domain.Group, error) {
	var updated *domain.Group

	err := s.pap.Write(ctx, func(tx storage.Tx, w *authz.PAPTx) error {
		existing, err := s.loadGroupForUpdate(ctx, tx, data.ID, data.Version)
		if err != nil {
			return err
		}

		oldPerms, err := w.GroupPermissions(existing.Name)
		if err != nil {
			return err
		}

		pAdded, pRemoved := sliceutil.Diff(oldPerms, data.Permissions)
		mAdded, mRemoved := sliceutil.Diff(existing.Members, data.Members)

		if err := s.authorizeUpdate(user, oldPerms, data.Permissions, pAdded, pRemoved, mAdded, mRemoved); err != nil {
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
		if err := w.ApplyPermissionDeltas(data.Name, pAdded, pRemoved); err != nil {
			return fmt.Errorf("apply permission deltas: %w", err)
		}
		if err := w.ApplyMemberDeltas(data.Name, mAdded, mRemoved); err != nil {
			return fmt.Errorf("apply member deltas: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update group: %w", err)
	}

	return updated, nil
}

// authorizeUpdate enforces the PDP boundary on a proposed group change.
//
// Two invariants:
//  1. Every permission entering or leaving the group must be within the
//     caller's own boundary (delta check).
//  2. If membership changes, every permission the group will hold post-update
//     must be within the caller's boundary (union check) — adding a member is
//     equivalent to granting them all of the group's permissions.
func (s *Service) authorizeUpdate(
	user domain.AuthInfo,
	oldPerms, newPerms []domain.Permission,
	pAdded, pRemoved []domain.Permission,
	mAdded, mRemoved []string,
) error {
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

	if len(mAdded) == 0 && len(mRemoved) == 0 {
		return nil
	}

	for _, p := range sliceutil.Union(oldPerms, newPerms) {
		if !s.pdp.Has(user.Email, p) {
			return domain.ErrPermissionEscalation
		}
	}

	return nil
}

func (s *Service) applyEntityUpdate(
	ctx context.Context,
	tx storage.Tx,
	existing *domain.Group,
	data UpdateData,
) error {
	existing.Name = data.Name
	existing.Description = data.Description
	existing.Members = data.Members
	existing.Version++
	existing.UpdatedAt = time.Now().UTC()

	if err := s.store.WithTx(tx).Update(ctx, existing); err != nil {
		return fmt.Errorf(errUpdateGroup, err)
	}

	return nil
}
