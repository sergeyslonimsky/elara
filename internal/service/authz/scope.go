package authz

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// GroupResolver is the narrow port Scope needs to translate Casbin group
// subjects (`group:<name>`) into persistent IDs and back. Lives here as
// an interface so authz stays free of bbolt imports — callers wire the
// usecase's existing reader (typically group.BoltGroupReader) at startup.
type GroupResolver interface {
	FindByName(ctx context.Context, name string) (*domain.Group, error)
}

// Scope encapsulates the derived authorization rules that span actor and
// target — "can actor X act on user/group Y?" — so usecase code stops
// re-implementing the same fan-out in three places.
//
// All methods are read-only. They observe whatever snapshot the underlying
// PDP/PAP currently see; callers that need a tx-consistent view should
// invoke them inside the surrounding PAP write transaction.
type Scope struct {
	pdp    *PDP
	pap    *PAP
	groups GroupResolver
}

// NewScope wires a Scope. groups should be the same GroupResolver used by
// the rest of the application so name→id resolution agrees with bbolt.
func NewScope(pdp *PDP, pap *PAP, groups GroupResolver) *Scope {
	return &Scope{pdp: pdp, pap: pap, groups: groups}
}

// CanReadUser reports whether actor can read target's profile.
//   - true if actor holds User:Read * (global), or
//   - true if target is a member of at least one group on which actor
//     holds Group:Read.
//
// Resolution errors (corrupted policy snapshot — group named in a g-rule
// but missing from bbolt) fail closed: the actor is denied. This mirrors
// CanWriteUser; for the imperative form that surfaces the error, see
// RequireWriteUser.
func (s *Scope) CanReadUser(ctx context.Context, actor, target string) bool {
	if s.pdp.HasGlobal(actor, domain.ObjectUser, domain.ActionRead) {
		return true
	}
	ok, _ := s.targetInGroupScope(ctx, actor, target, domain.ActionRead)

	return ok
}

// FilterVisibleUsers returns the subset of candidates the actor can see
// per CanReadUser, preserving order. Fast path: global User:Read returns
// the input as-is. Resolution errors per candidate are treated as "not
// visible" (fail closed) — consistent with CanReadUser.
func (s *Scope) FilterVisibleUsers(ctx context.Context, actor string, candidates []string) []string {
	if len(candidates) == 0 {
		return candidates
	}
	if s.pdp.HasGlobal(actor, domain.ObjectUser, domain.ActionRead) {
		return candidates
	}

	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if ok, _ := s.targetInGroupScope(ctx, actor, c, domain.ActionRead); ok {
			out = append(out, c)
		}
	}

	return out
}

// VisibleUserGroupIDs returns the IDs of target's groups on which actor
// holds Group:Read. Mirrors the GetUser response contract: memberships
// outside the actor's read scope are filtered out, not enumerated.
//
// Returns ([], nil) if the target belongs to no groups or to none the
// actor can read. Resolution errors propagate as wrapped errors.
func (s *Scope) VisibleUserGroupIDs(ctx context.Context, actor, target string) ([]string, error) {
	groups, err := s.resolveTargetGroups(ctx, target)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(groups))
	for _, g := range groups {
		if s.pdp.HasGroup(actor, g.ID, domain.ActionRead) {
			out = append(out, g.ID)
		}
	}

	return out, nil
}

// RequireMembershipGrant enforces the anti-escalation invariant for
// granting membership in `groupName` to a subject: the actor must hold
// every permission the group currently grants, because membership
// transitively bestows the full permission set on the new member.
//
// Reads the snapshot view through PAP, which is consistent with the
// pre-tx state of every flow that calls this helper (member add /
// initial-member grant) — none of them mutate the target group's
// permissions in the same write transaction. See PAP.Write docstring for
// the read-consistency contract.
//
// Removals narrow a subject's effective permissions and therefore require
// no escalation check; callers should not invoke this on the remove path.
func (s *Scope) RequireMembershipGrant(actor, groupName string) error {
	for _, p := range s.pap.GroupPermissions(groupName) {
		if !s.pdp.Has(actor, p) {
			return domain.ErrPermissionEscalation
		}
	}

	return nil
}

// RequireWriteUser is the imperative wrapper most callers want: same rule
// as CanWriteUser but returning domain.ErrForbidden when the actor's scope
// excludes the target. Resolution errors propagate as wrapped errors.
func (s *Scope) RequireWriteUser(ctx context.Context, actor, target string) error {
	if s.pdp.HasGlobal(actor, domain.ObjectUser, domain.ActionWrite) {
		return nil
	}
	ok, err := s.targetInGroupScope(ctx, actor, target, domain.ActionWrite)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}

	return nil
}

// resolveTargetGroups loads target's full group set in one place. Used by
// every method that needs to fan out over target's memberships
// (CanRead/CanWriteUser, FilterVisibleUsers, VisibleUserGroupIDs,
// RequireWriteUser). Pulling the UserGroupNames → FindByName loop into a
// single helper means a future change to the resolution path updates one
// site, not five.
//
// Resolution errors propagate to the caller; callers that prefer
// fail-closed-bool semantics simply discard the error.
func (s *Scope) resolveTargetGroups(ctx context.Context, target string) ([]*domain.Group, error) {
	names, err := s.pap.UserGroupNames(target)
	if err != nil {
		return nil, fmt.Errorf("pap user group names: %w", err)
	}

	out := make([]*domain.Group, 0, len(names))
	for _, name := range names {
		g, err := s.groups.FindByName(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("find group by name %s: %w", name, err)
		}
		out = append(out, g)
	}

	return out, nil
}

// targetInGroupScope reports whether actor holds `action` on at least one of
// target's groups. Resolution errors propagate; callers that want
// fail-closed-bool semantics discard them.
func (s *Scope) targetInGroupScope(
	ctx context.Context,
	actor, target string,
	action domain.Action,
) (bool, error) {
	groups, err := s.resolveTargetGroups(ctx, target)
	if err != nil {
		return false, err
	}
	for _, g := range groups {
		if s.pdp.HasGroup(actor, g.ID, action) {
			return true, nil
		}
	}

	return false, nil
}
