package authz

import (
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
)

// PAPTx is the per-transaction administration surface returned by PAP.Write.
// It speaks the domain language (group name, domain.Permission, email)
// instead of Casbin subject strings, so usecase code never has to know that
// a group is stored as `group:<name>` or that memberships live on the
// MembershipDomain g-rule.
type PAPTx struct {
	enforcer *casbin.Enforcer
	txe      *casbin.TxEnforcer
}

// GroupPermissions returns the permissions currently attached to the group,
// reading the in-tx policy snapshot so callers see writes made earlier in
// the same PAP.Write.
func (t *PAPTx) GroupPermissions(name string) ([]domain.Permission, error) {
	rules, err := t.txe.GetPermissionsForSubject(casbin.GroupSubject(name))
	if err != nil {
		return nil, fmt.Errorf("get group permissions: %w", err)
	}

	out := make([]domain.Permission, 0, len(rules))
	for _, r := range rules {
		out = append(out, domain.Permission{Domain: r[1], Object: r[2], Action: r[3]})
	}

	return out, nil
}

// ApplyPermissionDeltas removes the listed p-rules from the group then adds
// the new ones. The caller is responsible for diffing the desired state
// against the current state (typically via GroupPermissions).
func (t *PAPTx) ApplyPermissionDeltas(name string, added, removed []domain.Permission) error {
	subject := casbin.GroupSubject(name)

	for _, p := range removed {
		if err := t.txe.RemovePolicy(subject, p.Domain, p.Object, p.Action); err != nil {
			return fmt.Errorf("remove policy: %w", err)
		}
	}
	for _, p := range added {
		if err := t.txe.AddPolicy(subject, p.Domain, p.Object, p.Action); err != nil {
			return fmt.Errorf("add policy: %w", err)
		}
	}

	return nil
}

// ApplyMemberDeltas removes the listed users from the group then adds the
// new ones. Membership g-rules live on domain.MembershipDomain.
func (t *PAPTx) ApplyMemberDeltas(name string, added, removed []string) error {
	subject := casbin.GroupSubject(name)

	for _, email := range removed {
		if err := t.txe.RemoveRoleForUser(email, subject, domain.MembershipDomain); err != nil {
			return fmt.Errorf("remove membership: %w", err)
		}
	}
	for _, email := range added {
		if err := t.txe.AddRoleForUser(email, subject, domain.MembershipDomain); err != nil {
			return fmt.Errorf("add membership: %w", err)
		}
	}

	return nil
}

// DeleteGroup removes every Casbin rule attached to the group subject — its
// own permissions (p-rules), its role assignments, and all memberships
// pointing at it.
func (t *PAPTx) DeleteGroup(name string) error {
	if err := t.txe.DeleteUser(casbin.GroupSubject(name)); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	return nil
}

// DeleteUser removes every Casbin rule referencing the email, both as a
// subject (its role assignments / memberships) and as a target (none in the
// current model, but covered for symmetry).
func (t *PAPTx) DeleteUser(email string) error {
	if err := t.txe.DeleteUser(email); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}

// RenameGroup transfers every Casbin rule attached to the old group subject
// onto the new one. After this call the old subject has no remaining rules,
// and any subsequent ApplyPermissionDeltas / ApplyMemberDeltas against the
// new name behave exactly as if no rename had occurred. A no-op when
// oldName == newName.
//
// The rename must move three rule families:
//
//  1. p-rules where the group is the subject — its own permissions.
//     Read via t.txe.GetPermissionsForSubject(oldSubject); each row is
//     [sub, dom, obj, act]. Use t.txe.RemovePolicy / AddPolicy.
//
//  2. g-rules where the group is the subject — roles assigned TO the group
//     (e.g. `g, group:devs, admin, prod`). Read via
//     t.enforcer.GetRulesForSubject(oldSubject); each row is
//     [user, role, domain]. Use t.txe.RemoveRoleForUser / AddRoleForUser.
//
//  3. g-rules where the group is the target — memberships pointing AT the
//     group (e.g. `g, alice, group:devs, *`). Read members via
//     t.enforcer.GetMembersOfGroup(oldSubject); rebind each on
//     domain.MembershipDomain.
//
// The parent-enforcer reads are safe here because rename runs before any
// other mutation in the surrounding PAP.Write — the parent snapshot still
// matches the on-disk state for the old subject.
func (t *PAPTx) RenameGroup(oldName, newName string) error {
	if oldName == newName {
		return nil
	}

	oldSubject := casbin.GroupSubject(oldName)
	newSubject := casbin.GroupSubject(newName)

	pRules, err := t.txe.GetPermissionsForSubject(oldSubject)
	if err != nil {
		return fmt.Errorf("read group permissions: %w", err)
	}
	for _, r := range pRules {
		dom, obj, act := r[1], r[2], r[3]
		if err := t.txe.RemovePolicy(oldSubject, dom, obj, act); err != nil {
			return fmt.Errorf("rename: remove policy: %w", err)
		}
		if err := t.txe.AddPolicy(newSubject, dom, obj, act); err != nil {
			return fmt.Errorf("rename: add policy: %w", err)
		}
	}

	for _, r := range t.enforcer.GetRulesForSubject(oldSubject) {
		role, dom := r[1], r[2]
		if err := t.txe.RemoveRoleForUser(oldSubject, role, dom); err != nil {
			return fmt.Errorf("rename: remove group role: %w", err)
		}
		if err := t.txe.AddRoleForUser(newSubject, role, dom); err != nil {
			return fmt.Errorf("rename: add group role: %w", err)
		}
	}

	for _, email := range t.enforcer.GetMembersOfGroup(oldSubject) {
		if err := t.txe.RemoveRoleForUser(email, oldSubject, domain.MembershipDomain); err != nil {
			return fmt.Errorf("rename: remove membership: %w", err)
		}
		if err := t.txe.AddRoleForUser(email, newSubject, domain.MembershipDomain); err != nil {
			return fmt.Errorf("rename: add membership: %w", err)
		}
	}

	return nil
}
