package user

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	groupuc "github.com/sergeyslonimsky/elara/internal/usecase/group"
	"github.com/sergeyslonimsky/elara/internal/util/sliceutil"
)

// UpdateGroupsData carries the desired membership state for a single user.
// GroupIDs is the canonical desired set: the server diffs against current
// memberships and only operates on the symmetric difference.
type UpdateGroupsData struct {
	Email    string
	GroupIDs []string
}

// UpdateGroupsResult mirrors the post-update state. GroupIDs equals
// data.GroupIDs on success and is included so callers don't need to re-fetch.
type UpdateGroupsResult struct {
	User     *domain.User
	GroupIDs []string
}

// UpdateGroups replaces the target user's group memberships with the given
// set of group IDs.
//
// The handler is responsible for the coarse authentication and the
// resource-class authz gate (ObjectUser:Write plus ObjectGroup:Write when
// the request carries any groups). This method enforces the fine-grained
// per-delta checks inside the same transaction as the Casbin g-rule sync:
//
//  1. For every added or removed group, the actor must hold
//     ObjectGroup:Write on "group:<id>". Groups unchanged between current and
//     desired state require no permission, so a UI that returns read-only
//     groups untouched is naturally allowed.
//  2. For every added group, the actor must hold every permission the group
//     grants (anti-escalation; see groupuc.AuthorizeGrantToUser).
//
// All PDP reads inside the closure observe a stable policy snapshot:
// casbin.Enforcer serializes WriteTx, so no concurrent policy mutation can
// commit between authorization and the membership write — this is why a
// TOCTOU window doesn't exist here.
//
// The bbolt User record is unchanged — group memberships live exclusively in
// Casbin g-rules of the form `g, <email>, group:<name>, "*"`.
func (s *Service) UpdateGroups(
	ctx context.Context,
	actor domain.AuthInfo,
	data UpdateGroupsData,
) (*UpdateGroupsResult, error) {
	var result *UpdateGroupsResult

	err := s.enforcer.WriteTx(ctx, s.txm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
		user, err := s.users.WithTx(tx).Get(ctx, data.Email)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		groups := s.groups.WithTx(tx)

		desired, err := loadGroupsByIDs(ctx, groups, data.GroupIDs)
		if err != nil {
			return err
		}

		currentIDs, currentNamesByID, err := currentUserGroupIDs(ctx, s.enforcer, groups, user.Email)
		if err != nil {
			return err
		}

		added, removed := sliceutil.Diff(currentIDs, data.GroupIDs)

		if err := s.authorizeGroupDeltas(actor, added, removed); err != nil {
			return err
		}

		for _, id := range added {
			if err := groupuc.AuthorizeGrantToUser(s.pdp, txe, actor, desired[id].Name); err != nil {
				return fmt.Errorf("grant group %s: %w", id, err)
			}
		}

		if err := applyMembershipDeltas(txe, user.Email, added, removed, desired, currentNamesByID); err != nil {
			return err
		}

		result = &UpdateGroupsResult{User: user, GroupIDs: data.GroupIDs}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update user groups: %w", err)
	}

	return result, nil
}

func (s *Service) authorizeGroupDeltas(actor domain.AuthInfo, added, removed []string) error {
	for _, id := range added {
		if !s.pdp.Has(actor.Email, domain.Permission{
			Object: domain.ObjectGroup,
			Action: domain.ActionWrite,
			Domain: "group:" + id,
		}) {
			return domain.ErrForbidden
		}
	}

	for _, id := range removed {
		if !s.pdp.Has(actor.Email, domain.Permission{
			Object: domain.ObjectGroup,
			Action: domain.ActionWrite,
			Domain: "group:" + id,
		}) {
			return domain.ErrForbidden
		}
	}

	return nil
}

// loadGroupsByIDs fetches each requested group inside the current tx and
// returns them keyed by ID. A missing or invalid ID surfaces as the repo's
// Get error and aborts the transaction.
func loadGroupsByIDs(ctx context.Context, repo GroupReader, ids []string) (map[string]*domain.Group, error) {
	out := make(map[string]*domain.Group, len(ids))
	for _, id := range ids {
		g, err := repo.Get(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get group %s: %w", id, err)
		}

		out[id] = g
	}

	return out, nil
}

// currentUserGroupIDs reads the user's current Casbin g-rules and resolves
// each group-subject to its ID via FindByName. Nested-group memberships and
// non-group subjects are skipped, matching the convention in
// usecase/user/service_list.go.
func currentUserGroupIDs(
	ctx context.Context,
	enforcer *casbin.Enforcer,
	repo GroupReader,
	email string,
) ([]string, map[string]string, error) {
	subjects, err := enforcer.GetRolesForUser(email, domain.MembershipDomain)
	if err != nil {
		return nil, nil, fmt.Errorf("get user memberships: %w", err)
	}

	ids := make([]string, 0, len(subjects))
	namesByID := make(map[string]string, len(subjects))

	for _, subject := range subjects {
		if !casbin.IsGroupSubject(subject) {
			continue
		}

		name := casbin.GroupNameFromSubject(subject)

		g, err := repo.FindByName(ctx, name)
		if err != nil {
			return nil, nil, fmt.Errorf("find group by name %s: %w", name, err)
		}

		ids = append(ids, g.ID)
		namesByID[g.ID] = g.Name
	}

	return ids, namesByID, nil
}

// applyMembershipDeltas applies the resolved add/remove deltas to Casbin
// g-rules. Names are looked up via the per-state maps populated upstream so
// we never re-query bbolt inside the inner loop.
func applyMembershipDeltas(
	txe *casbin.TxEnforcer,
	email string,
	added, removed []string,
	desired map[string]*domain.Group,
	currentNamesByID map[string]string,
) error {
	for _, id := range removed {
		name := currentNamesByID[id]
		if err := txe.RemoveRoleForUser(email, casbin.GroupSubject(name), domain.MembershipDomain); err != nil {
			return fmt.Errorf("remove membership %s: %w", id, err)
		}
	}

	for _, id := range added {
		name := desired[id].Name
		if err := txe.AddRoleForUser(email, casbin.GroupSubject(name), domain.MembershipDomain); err != nil {
			return fmt.Errorf("add membership %s: %w", id, err)
		}
	}

	return nil
}
