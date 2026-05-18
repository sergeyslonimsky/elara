package group

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// Authorization for all GroupService methods is enforced by the RBAC
// interceptor (object="group", action=read|write at DomainAll) before any
// of these methods runs — see internal/handler/v2/interceptor/rbac_policy.go.

func (s *Service) Create(ctx context.Context, name string) (*domain.Group, error) {
	now := time.Now().UTC()
	group := &domain.Group{
		ID:        uuid.New().String(),
		Name:      name,
		Members:   []string{},
		Version:   1, // Starting version
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.Create(ctx, group); err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}

	return group, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Group, error) {
	group, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	return group, nil
}

func (s *Service) Update(
	ctx context.Context,
	id string,
	name string,
	description string,
	permissions []domain.Permission,
	members []string,
	version int64,
) (*domain.Group, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	principal := claims.Email

	var updatedGroup *domain.Group

	err := s.enforcer.WriteTx(ctx, s.txm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
		existing, err := s.store.WithTx(tx).Get(ctx, id)
		if err != nil {
			return fmt.Errorf(errGetGroup, err)
		}

		if existing.Version != version {
			return domain.ErrVersionConflict
		}

		if err := existing.EnsureMutable(); err != nil {
			return err
		}

		// 1. Compute Permission Delta & Check Boundary
		rules, err := txe.GetPermissionsForSubject(domain.GroupSubject(existing.Name))
		if err != nil {
			return fmt.Errorf("get permissions: %w", err)
		}

		var oldPerms []domain.Permission
		for _, rule := range rules {
			oldPerms = append(oldPerms, domain.Permission{
				Object: rule[2],
				Action: rule[3],
				Domain: rule[1],
			})
		}

		pAdded, pRemoved := diffPermissions(oldPerms, permissions)
		pDelta := append(pAdded, pRemoved...)

		for _, p := range pDelta {
			if !s.pdp.Has(principal, p) {
				return domain.ErrPermissionEscalation
			}
		}

		// 2. Compute Member Delta & Check Union Rule
		mAdded, mRemoved := diffStrings(existing.Members, members)
		if len(mAdded) > 0 || len(mRemoved) > 0 {
			union := unionPermissions(oldPerms, permissions)
			for _, p := range union {
				if !s.pdp.Has(principal, p) {
					return domain.ErrPermissionEscalation
				}
			}
		}

		// Prepare updated group entity
		oldName := existing.Name
		existing.Name = name
		existing.Description = description
		existing.Members = members
		existing.Version++
		existing.UpdatedAt = time.Now().UTC()

		if err := s.store.WithTx(tx).Update(ctx, existing); err != nil {
			return fmt.Errorf(errUpdateGroup, err)
		}

		updatedGroup = existing

		oldSubject := domain.GroupSubject(oldName)
		newSubject := domain.GroupSubject(name)

		// Handle renaming if necessary
		if oldName != name {
			// Reassign the group's own role rules: g, group:<old>, <role>, <dom> -> g, group:<new>, <role>, <dom>
			for _, rule := range s.enforcer.GetRulesForSubject(oldSubject) {
				if err := txe.AddRoleForUser(newSubject, rule[1], rule[2]); err != nil {
					return err
				}
				if err := txe.RemoveRoleForUser(oldSubject, rule[1], rule[2]); err != nil {
					return err
				}
			}
		}

		// Update Permissions (p-rules)
		for _, p := range pRemoved {
			if err := txe.RemovePolicy(newSubject, p.Domain, p.Object, p.Action); err != nil {
				return err
			}
		}
		for _, p := range pAdded {
			if err := txe.AddPolicy(newSubject, p.Domain, p.Object, p.Action); err != nil {
				return err
			}
		}

		// Update Members (g-rules)
		// If renamed, we must remove ALL old memberships and add ALL new ones.
		// If NOT renamed, we only add/remove the delta.
		if oldName != name {
			// Remove all old memberships
			for _, m := range s.enforcer.GetMembersOfGroup(oldSubject) {
				if err := txe.RemoveRoleForUser(m, oldSubject, domain.MembershipDomain); err != nil {
					return err
				}
			}
			// Add all new memberships
			for _, m := range members {
				if err := txe.AddRoleForUser(m, newSubject, domain.MembershipDomain); err != nil {
					return err
				}
			}
		} else {
			for _, m := range mRemoved {
				if err := txe.RemoveRoleForUser(m, newSubject, domain.MembershipDomain); err != nil {
					return err
				}
			}
			for _, m := range mAdded {
				if err := txe.AddRoleForUser(m, newSubject, domain.MembershipDomain); err != nil {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return updatedGroup, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	group, err := s.store.Get(ctx, id)
	if err != nil {
		return fmt.Errorf(errGetGroup, err)
	}

	if group.System {
		return fmt.Errorf("delete system group: %w", domain.ErrForbidden)
	}

	err = s.enforcer.WriteTx(ctx, s.txm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
		if err := txe.DeleteUser(domain.GroupSubject(group.Name)); err != nil {
			return fmt.Errorf("delete group from casbin: %w", err)
		}

		if err := s.store.WithTx(tx).Delete(ctx, id); err != nil {
			return fmt.Errorf("delete group: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) List(ctx context.Context) ([]*domain.Group, error) {
	groups, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	return groups, nil
}
