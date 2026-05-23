package authz

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

const gRuleNativeLen = 3

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

// GroupRoleAssignment is a canonical view of a `g, group:<name>, role, dom`
// rule with the group: prefix already stripped from the subject.
type GroupRoleAssignment struct {
	Group  string
	Role   string
	Domain string
}

// ListGroupRoleAssignments returns every g-rule whose subject is a group,
// projected into domain language. Direct user grants and membership rules
// are filtered out — only group → role bindings are surfaced. Order is
// unspecified.
func (p *PAP) ListGroupRoleAssignments() []GroupRoleAssignment {
	rules := p.enforcer.GetGroupingPolicy()
	result := make([]GroupRoleAssignment, 0, len(rules))

	for _, rule := range rules {
		if len(rule) < gRuleNativeLen {
			continue
		}
		if !casbin.IsGroupSubject(rule[0]) {
			continue
		}

		result = append(result, GroupRoleAssignment{
			Group:  casbin.GroupNameFromSubject(rule[0]),
			Role:   rule[1],
			Domain: rule[2],
		})
	}

	return result
}

// AdminAssignmentCount returns the total number of g-rules that grant
// domain.RoleAdmin on domain.DomainAll, across both direct user grants and
// group grants. Used by the last-admin guard so the caller can compare it
// against HasDirectAdminAssignment.
func (p *PAP) AdminAssignmentCount() int {
	count := 0

	for _, rule := range p.enforcer.GetGroupingPolicy() {
		if len(rule) == gRuleNativeLen && rule[1] == domain.RoleAdmin && rule[2] == domain.DomainAll {
			count++
		}
	}

	return count
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

// GroupMembers returns the emails of users currently in the given group,
// reading from the in-memory Casbin snapshot. Nested-group subjects are
// filtered out (matching the convention used by MembersOfScope) so the
// returned set contains only user emails. Order is unspecified.
//
// Casbin is the source of truth for membership — bbolt does not persist
// this relation. See domain.Group.Members for the transient view.
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

// HasDirectAdminAssignment reports whether the given email holds a g-rule
// directly granting domain.RoleAdmin on domain.DomainAll (i.e. not via group
// membership). Used to detect the bootstrap-admin / single-user-admin case.
func (p *PAP) HasDirectAdminAssignment(email string) bool {
	for _, rule := range p.enforcer.GetGroupingPolicy() {
		if len(rule) == gRuleNativeLen &&
			rule[0] == email &&
			rule[1] == domain.RoleAdmin &&
			rule[2] == domain.DomainAll {
			return true
		}
	}

	return false
}
