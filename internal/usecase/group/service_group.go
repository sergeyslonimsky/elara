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

	if err := group.Validate(); err != nil {
		return nil, fmt.Errorf("validate group: %w", err)
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

func (s *Service) Update( //nolint:cyclop,funlen //refactor
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
			return fmt.Errorf("ensure mutable: %w", err)
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
		pDelta := make([]domain.Permission, 0, len(pAdded)+len(pRemoved))
		pDelta = append(pDelta, pAdded...)
		pDelta = append(pDelta, pRemoved...)

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
					return fmt.Errorf("add role for user: %w", err)
				}
				if err := txe.RemoveRoleForUser(oldSubject, rule[1], rule[2]); err != nil {
					return fmt.Errorf("remove role for user: %w", err)
				}
			}
		}

		// Update Permissions (p-rules)
		for _, p := range pRemoved {
			if err := txe.RemovePolicy(newSubject, p.Domain, p.Object, p.Action); err != nil {
				return fmt.Errorf("remove policy: %w", err)
			}
		}
		for _, p := range pAdded {
			if err := txe.AddPolicy(newSubject, p.Domain, p.Object, p.Action); err != nil {
				return fmt.Errorf("add policy: %w", err)
			}
		}

		// Update Members (g-rules)
		// If renamed, we must remove ALL old memberships and add ALL new ones.
		// If NOT renamed, we only add/remove the delta.
		if oldName != name {
			// Remove all old memberships
			for _, m := range s.enforcer.GetMembersOfGroup(oldSubject) {
				if err := txe.RemoveRoleForUser(m, oldSubject, domain.MembershipDomain); err != nil {
					return fmt.Errorf("remove role for user: %w", err)
				}
			}
			// Add all new memberships
			for _, m := range members {
				if err := txe.AddRoleForUser(m, newSubject, domain.MembershipDomain); err != nil {
					return fmt.Errorf("add role for user: %w", err)
				}
			}
		} else {
			for _, m := range mRemoved {
				if err := txe.RemoveRoleForUser(m, newSubject, domain.MembershipDomain); err != nil {
					return fmt.Errorf("remove role from user: %w", err)
				}
			}
			for _, m := range mAdded {
				if err := txe.AddRoleForUser(m, newSubject, domain.MembershipDomain); err != nil {
					return fmt.Errorf("add role for user: %w", err)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("write tx: %w", err)
	}

	return updatedGroup, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	err := s.enforcer.WriteTx(ctx, s.txm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
		group, err := s.store.WithTx(tx).Get(ctx, id)
		if err != nil {
			return fmt.Errorf(errGetGroup, err)
		}

		if err := group.EnsureMutable(); err != nil {
			return fmt.Errorf("ensure mutable: %w", err)
		}

		if err := txe.DeleteUser(domain.GroupSubject(group.Name)); err != nil {
			return fmt.Errorf("delete group from casbin: %w", err)
		}

		if err := s.store.WithTx(tx).Delete(ctx, id); err != nil {
			return fmt.Errorf("delete group: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("write tx: %w", err)
	}

	return nil
}

// List returns groups the authenticated caller can read.
//
// Filtering happens at the repository: the caller's effective group set
// (from PDP.EffectiveDomains for object=group action=read) is translated
// into a GroupFilter — no post-fetch pdp.Has loop. An empty effective set
// returns an empty list, not an error (EL-4 §7 acceptance: empty
// responses → empty list, not 403).
func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	scope := s.pdp.EffectiveDomains(claims.Email, domain.ObjectGroup, domain.ActionRead)
	if scope.IsEmpty() {
		return &ListResult{
			Groups: []*domain.Group{},
			Total:  0,
			Limit:  limit,
			Offset: params.Offset,
		}, nil
	}

	// EffectiveDomains for object=group yields domains in the "group:<name>"
	// subject form. Strip the prefix; non-group entries (defensive) are
	// ignored — Wildcard already covers them.
	names := make(map[string]struct{}, len(scope.Explicit))
	for d := range scope.Explicit {
		if domain.IsGroupSubject(d) {
			names[domain.GroupNameFromSubject(d)] = struct{}{}
		}
	}

	filter := domain.GroupFilter{
		Names:    names,
		Wildcard: scope.Wildcard,
		Search:   params.Query,
	}
	repoParams := domain.GroupListParams{
		Limit:  limit,
		Offset: params.Offset,
		Sort:   params.Sort,
	}

	groups, total, err := s.store.List(ctx, filter, repoParams)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	return &ListResult{
		Groups: groups,
		Total:  total,
		Limit:  limit,
		Offset: params.Offset,
	}, nil
}
