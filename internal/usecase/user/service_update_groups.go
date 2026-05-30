package user

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
	"github.com/sergeyslonimsky/elara/internal/util/sliceutil"
)

// UpdateGroupsData carries the explicit membership delta for one user.
// Adding a group the user already belongs to is a no-op; removing one they
// aren't in is a no-op. Same id in both add and remove returns
// InvalidArgument.
type UpdateGroupsData struct {
	Email                     string
	AddGroupIDs               []string
	RemoveGroupIDs            []string
	ExpectedMembershipVersion *int64
}

// UpdateGroupsResult mirrors the post-update state. VisibleGroupIDs is
// scope-filtered the same way GetUser's result is — out-of-scope
// memberships are not exposed so the response can't be used to enumerate
// groups the caller cannot read.
type UpdateGroupsResult struct {
	User              *domain.User
	VisibleGroupIDs   []string
	MembershipVersion int64
}

// UpdateGroups applies an explicit add/remove delta to the target user's
// group memberships.
//
// Authorization (per id in AddGroupIDs ∪ RemoveGroupIDs):
//   - actor must hold Group:Write on the group.
//
// Anti-escalation: for each effective add, the actor must hold every
// permission the group currently grants. Removals narrow and skip the
// escalation check.
//
// Optimistic concurrency: if ExpectedMembershipVersion is set and current
// User.MembershipVersion differs, returns domain.ErrVersionConflict even
// when the net delta would be a no-op.
func (s *Service) UpdateGroups(
	ctx context.Context,
	actor domain.AuthInfo,
	data UpdateGroupsData,
) (*UpdateGroupsResult, error) {
	if id, dup := sliceutil.FirstOverlap(data.AddGroupIDs, data.RemoveGroupIDs); dup {
		return nil, domain.NewValidationError("group_id", fmt.Sprintf("%q appears in both add and remove", id))
	}

	var result *UpdateGroupsResult

	err := s.pap.Write(ctx, func(tx storage.Tx, w *authz.PAPTx) error {
		user, err := s.store.WithTx(tx).Get(ctx, data.Email)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if err := domain.CheckVersion(data.ExpectedMembershipVersion, user.MembershipVersion); err != nil {
			return fmt.Errorf("check version: %w", err)
		}

		delta, err := s.computeGroupDelta(ctx, tx, actor, user.Email, data)
		if err != nil {
			return err
		}

		// `visible_group_ids` is scope-filtered so the response cannot
		// enumerate memberships outside the caller's read scope.
		visible := filterVisibleGroupIDs(s.pdp, actor.Email, delta.postIDs)

		if len(delta.addedNames)+len(delta.removedNames) == 0 {
			// No-op apply but optimistic-lock check already passed; return
			// current state without bumping the counter.
			result = &UpdateGroupsResult{
				User:              user,
				VisibleGroupIDs:   visible,
				MembershipVersion: user.MembershipVersion,
			}

			return nil
		}

		if err := s.applyMembershipDelta(ctx, tx, w, user, delta.addedNames, delta.removedNames); err != nil {
			return err
		}

		result = &UpdateGroupsResult{
			User:              user,
			VisibleGroupIDs:   visible,
			MembershipVersion: user.MembershipVersion,
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update user groups: %w", err)
	}

	return result, nil
}

// groupDelta carries the resolved membership delta UpdateGroups needs to
// apply the change and render the post-state response. Lives next to its
// only producer/consumer so the field meaning stays close to the logic.
type groupDelta struct {
	addedNames   []string
	removedNames []string
	postIDs      []string
}

// computeGroupDelta resolves the requested add/remove union into casbin
// names and the post-state id list, enforcing both the Group:Write
// authorization (per requested id) and the anti-escalation invariant (per
// effectively added group). Extracted from UpdateGroups so the orchestrator
// stays under the cyclop threshold.
func (s *Service) computeGroupDelta(
	ctx context.Context,
	tx storage.Tx,
	actor domain.AuthInfo,
	email string,
	data UpdateGroupsData,
) (groupDelta, error) {
	groups := s.groups.WithTx(tx)

	currentIDs, currentNamesByID, err := currentUserGroupIDs(ctx, s.pap, groups, email)
	if err != nil {
		return groupDelta{}, err
	}
	// Authorize on the requested union (matches proto contract): a caller
	// who lacks Group:Write on X must not be able to submit "remove X" and
	// silently learn that the target isn't in X (no-op success =
	// enumeration oracle).
	if err := s.authorizeGroupDeltas(actor, data.AddGroupIDs, data.RemoveGroupIDs); err != nil {
		return groupDelta{}, err
	}

	currentSet := sliceutil.ToSet(currentIDs)
	effectiveAdd := sliceutil.NotIn(data.AddGroupIDs, currentSet)
	effectiveRemove := sliceutil.In(data.RemoveGroupIDs, currentSet)

	// Resolve added group ids to (id -> group) for anti-escalation and for
	// the casbin-name needed by ApplyUserMembershipDeltas.
	addedGroups, err := loadGroupsByIDs(ctx, groups, effectiveAdd)
	if err != nil {
		return groupDelta{}, err
	}

	addedNames := make([]string, 0, len(effectiveAdd))
	for _, id := range effectiveAdd {
		if err := s.scope.RequireMembershipGrant(actor.Email, addedGroups[id].Name); err != nil {
			return groupDelta{}, fmt.Errorf("grant group %s: %w", id, err)
		}
		addedNames = append(addedNames, addedGroups[id].Name)
	}

	removedNames := make([]string, 0, len(effectiveRemove))
	for _, id := range effectiveRemove {
		removedNames = append(removedNames, currentNamesByID[id])
	}

	return groupDelta{
		addedNames:   addedNames,
		removedNames: removedNames,
		postIDs:      sliceutil.ComposePost(currentIDs, effectiveAdd, effectiveRemove),
	}, nil
}

// applyMembershipDelta writes the casbin g-rule change and persists the
// bumped MembershipVersion. Order matters — the version is the optimistic
// lock seed for subsequent UpdateGroups calls — so the steps must stay in
// this sequence inside the surrounding PAP write transaction.
func (s *Service) applyMembershipDelta(
	ctx context.Context,
	tx storage.Tx,
	w *authz.PAPTx,
	user *domain.User,
	addedNames, removedNames []string,
) error {
	if err := w.ApplyUserMembershipDeltas(user.Email, addedNames, removedNames); err != nil {
		return fmt.Errorf("pap apply user memberships: %w", err)
	}
	user.MembershipVersion++
	if err := s.store.WithTx(tx).SetMembershipVersion(ctx, user.Email, user.MembershipVersion); err != nil {
		return fmt.Errorf("persist membership version: %w", err)
	}

	return nil
}

func (s *Service) authorizeGroupDeltas(actor domain.AuthInfo, added, removed []string) error {
	for _, ids := range [...][]string{added, removed} {
		for _, id := range ids {
			if !s.pdp.HasGroup(actor.Email, id, domain.ActionWrite) {
				return domain.ErrForbidden
			}
		}
	}

	return nil
}

// filterVisibleGroupIDs returns the subset of ids on which actor holds
// Group:Read, preserving order. Used to scope GetUser/UpdateGroups
// responses so the proto's `visible_group_ids` field can't enumerate
// memberships outside the caller's read scope.
func filterVisibleGroupIDs(pdp *authz.PDP, actor string, ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if pdp.HasGroup(actor, id, domain.ActionRead) {
			out = append(out, id)
		}
	}

	return out
}

// loadGroupsByIDs fetches each requested group inside the current tx and
// returns them keyed by ID.
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

// currentUserGroupIDs reads current memberships through PAP and resolves
// each group name to its ID via the per-tx GroupReader.
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
