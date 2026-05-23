package user

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
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
// per-delta checks inside the same transaction as the membership write:
//
//  1. For every added or removed group, the actor must hold
//     ObjectGroup:Write on "group:<id>". Groups unchanged between current and
//     desired state require no permission, so a UI that returns read-only
//     groups untouched is naturally allowed.
//  2. For every added group, the actor must hold every permission the group
//     grants (anti-escalation; see groupuc.AuthorizeGrantToUser).
//
// All PDP reads inside the closure observe a stable policy snapshot: PAP
// serializes Write, so no concurrent policy mutation can commit between
// authorization and the membership write — this is why a TOCTOU window
// doesn't exist here.
//
// The bbolt User record is unchanged — group memberships live exclusively in
// Casbin g-rules of the form `g, <email>, group:<name>, "*"`.
func (s *Service) UpdateGroups(
	ctx context.Context,
	actor domain.AuthInfo,
	data UpdateGroupsData,
) (*UpdateGroupsResult, error) {
	var result *UpdateGroupsResult

	err := s.pap.Write(ctx, func(tx storage.Tx, w *authz.PAPTx) error {
		user, err := s.users.WithTx(tx).Get(ctx, data.Email)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		groups := s.groups.WithTx(tx)

		desired, err := loadGroupsByIDs(ctx, groups, data.GroupIDs)
		if err != nil {
			return err
		}

		currentIDs, currentNamesByID, err := currentUserGroupIDs(ctx, s.pap, groups, user.Email)
		if err != nil {
			return err
		}

		added, removed := sliceutil.Diff(currentIDs, data.GroupIDs)

		if err := s.authorizeGroupDeltas(actor, added, removed); err != nil {
			return err
		}

		for _, id := range added {
			if err := groupuc.AuthorizeGrantToUser(s.pdp, w, actor, desired[id].Name); err != nil {
				return fmt.Errorf("grant group %s: %w", id, err)
			}
		}

		addedNames := mapIDsToNames(added, func(id string) string { return desired[id].Name })
		removedNames := mapIDsToNames(removed, func(id string) string { return currentNamesByID[id] })

		if err := w.ApplyUserMembershipDeltas(user.Email, addedNames, removedNames); err != nil {
			return fmt.Errorf("pap apply user memberships: %w", err)
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

// currentUserGroupIDs reads the user's current group memberships through PAP
// and resolves each group name to its ID via the per-tx GroupReader. The
// caller gets both the ordered ID slice (for diffing) and a name lookup
// keyed by ID (for the removal path, which needs the casbin subject name).
func currentUserGroupIDs(
	ctx context.Context,
	pap *authz.PAP,
	repo GroupReader,
	email string,
) ([]string, map[string]string, error) {
	names, err := pap.UserGroupNames(email)
	if err != nil {
		return nil, nil, fmt.Errorf("pap user group names: %w", err)
	}

	ids := make([]string, 0, len(names))
	namesByID := make(map[string]string, len(names))

	for _, name := range names {
		g, err := repo.FindByName(ctx, name)
		if err != nil {
			return nil, nil, fmt.Errorf("find group by name %s: %w", name, err)
		}

		ids = append(ids, g.ID)
		namesByID[g.ID] = g.Name
	}

	return ids, namesByID, nil
}

func mapIDsToNames(ids []string, lookup func(string) string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, lookup(id))
	}

	return out
}
