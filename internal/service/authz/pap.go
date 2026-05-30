package authz

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// PAP (Policy Administration Point) is the write-side complement of PDP.
//
// PDP answers "can principal X do Y on Z?" without exposing Casbin to its
// callers. PAP applies group-level mutations (rename, permission deltas,
// membership deltas, delete) without exposing Casbin's subject-string
// convention or p/g-rule shape either. All mutations run inside Casbin's
// transactional WriteTx so they commit atomically with the bbolt write of
// any companion entity.
type PAP struct {
	enforcer *casbin.Enforcer
	txm      storage.TxManager
}

func NewPAP(enforcer *casbin.Enforcer, txm storage.TxManager) *PAP {
	return &PAP{enforcer: enforcer, txm: txm}
}

// Write opens a Casbin write transaction and hands the per-tx administration
// surface to fn. The storage.Tx is exposed so callers can combine PAP
// mutations with bbolt repo writes in the same atomic step.
//
// Read consistency inside fn:
//   - Mutations made through `w *PAPTx` go to the per-tx PolicyRepo and
//     are NOT visible to PDP / parent-PAP reads until commit (applyOpsToCache).
//   - PDP.Has, PAP.GroupPermissions, PAP.GroupMembers and similar all read
//     the pre-tx in-memory snapshot. That snapshot is stable for the duration
//     of the Write because PAP.Write holds the bbolt write lock — no other
//     writer can interleave.
//   - Practical rule: if you intend to mutate X and then re-check X in the
//     same Write, route the read through `w` (e.g. PAPTx.GroupPermissions).
//     For checks against unchanged subjects (today: anti-escalation against
//     the actor's own perms or the target group's pre-existing perms), the
//     parent-snapshot helpers are safe.
func (p *PAP) Write(
	ctx context.Context,
	fn func(tx storage.Tx, w *PAPTx) error,
) error {
	err := p.enforcer.WriteTx(ctx, p.txm, func(tx storage.Tx, txe *casbin.TxEnforcer) error {
		return fn(tx, &PAPTx{enforcer: p.enforcer, txe: txe})
	})
	if err != nil {
		return fmt.Errorf("pap write: %w", err)
	}

	return nil
}

// GroupNamesFromScope extracts group names from the scope's explicit
// domains. Domains that are not group subjects are skipped — callers should
// inspect scope.Wildcard separately when they need a "see everything"
// fast-path.
func (p *PAP) GroupNamesFromScope(scope DomainSet) map[string]struct{} {
	names := make(map[string]struct{}, len(scope.Explicit))
	for d := range scope.Explicit {
		if casbin.IsGroupSubject(d) {
			names[casbin.GroupNameFromSubject(d)] = struct{}{}
		}
	}

	return names
}

// MembersOfScope returns the set of user emails who are members of at least
// one group in the scope's explicit domains. Used to translate a
// "groups I can see" scope into a "users I can see" filter. Nested-group
// members are filtered out — see EL-4 §1.
func (p *PAP) MembersOfScope(scope DomainSet) map[string]struct{} {
	users := make(map[string]struct{})
	for d := range scope.Explicit {
		if !casbin.IsGroupSubject(d) {
			continue
		}
		for _, m := range p.enforcer.GetMembersOfGroup(d) {
			if !casbin.IsGroupSubject(m) {
				users[m] = struct{}{}
			}
		}
	}

	return users
}

// GroupPermissions returns the permission set currently attached to the
// group, reading from the in-memory Casbin snapshot. Use this when only a
// read is needed (e.g. composing GetGroup / ListGroups responses) and
// inside a PAP.Write closure only when the surrounding code does NOT
// mutate the same group's p-rules — otherwise call PAPTx.GroupPermissions
// to observe in-tx state. See PAP.Write docstring for the read-consistency
// contract.
//
// p-rules are filtered to length 4 (subject, dom, obj, act); shorter rows
// are skipped defensively. Order is unspecified.
func (p *PAP) GroupPermissions(name string) []domain.Permission {
	subject := casbin.GroupSubject(name)

	out := make([]domain.Permission, 0)
	for _, r := range p.enforcer.GetPolicy() {
		if len(r) < pRuleLen || r[0] != subject {
			continue
		}
		out = append(out, domain.Permission{Domain: r[1], Object: domain.Object(r[2]), Action: domain.Action(r[3])})
	}

	return out
}

// GroupMembers returns the emails of users currently in the given group,
// reading from the in-memory Casbin snapshot. Nested-group subjects are
// filtered out (matching the convention used by MembersOfScope) so the
// returned set contains only user emails. Order is unspecified.
//
// Casbin is the source of truth for membership — bbolt does not persist
// this relation.
func (p *PAP) GroupMembers(name string) []string {
	raw := p.enforcer.GetMembersOfGroup(casbin.GroupSubject(name))
	out := make([]string, 0, len(raw))
	for _, m := range raw {
		if !casbin.IsGroupSubject(m) {
			out = append(out, m)
		}
	}

	return out
}

// UserGroupNames returns the names of groups the user currently belongs to,
// reading from the in-memory policy snapshot. Nested-group memberships and
// non-group subjects (legacy direct user→role bindings) are skipped, matching
// the convention used by the list-scoping code.
func (p *PAP) UserGroupNames(email string) ([]string, error) {
	subjects, err := p.enforcer.GetRolesForUser(email, domain.MembershipDomain)
	if err != nil {
		return nil, fmt.Errorf("get user memberships: %w", err)
	}

	out := make([]string, 0, len(subjects))
	for _, subject := range subjects {
		if !casbin.IsGroupSubject(subject) {
			continue
		}
		out = append(out, casbin.GroupNameFromSubject(subject))
	}

	return out, nil
}
