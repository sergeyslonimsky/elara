package authz

import (
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
)

// PAPTx is the per-transaction administration surface returned by PAP.Write.
// It speaks the domain language (group name, domain.Permission, user ID)
// instead of Casbin subject strings, so usecase code never has to know that
// a group is stored as `group:<name>` or that memberships live on the
// MembershipDomain g-rule.
type PAPTx struct {
	txe *casbin.TxEnforcer
}

// GroupPermissions returns the permissions currently attached to the group,
// reading the in-tx policy snapshot so callers see writes made earlier in
// the same PAP.Write.
func (t *PAPTx) GroupPermissions(name string) ([]domain.Permission, error) {
	rules, err := t.txe.GetPermissionsForSubject(domain.GroupResource(name))
	if err != nil {
		return nil, fmt.Errorf("get group permissions: %w", err)
	}

	out := make([]domain.Permission, 0, len(rules))
	for _, r := range rules {
		out = append(
			out,
			domain.Permission{
				Domain: r[1],
				Object: domain.Object(r[2]),
				Action: domain.Action(r[3]),
			},
		)
	}

	return out, nil
}

// ApplyPermissionDeltas removes the listed p-rules from the group then adds
// the new ones. The caller is responsible for diffing the desired state
// against the current state (typically via GroupPermissions).
func (t *PAPTx) ApplyPermissionDeltas(name string, added, removed []domain.Permission) error {
	subject := domain.GroupResource(name)

	for _, p := range removed {
		if err := t.txe.RemovePolicy(subject, p.Domain, string(p.Object), string(p.Action)); err != nil {
			return fmt.Errorf("remove policy: %w", err)
		}
	}
	for _, p := range added {
		if err := t.txe.AddPolicy(subject, p.Domain, string(p.Object), string(p.Action)); err != nil {
			return fmt.Errorf("add policy: %w", err)
		}
	}

	return nil
}

// ApplyMemberDeltas removes the listed users from the group then adds the
// new ones. Membership g-rules live on domain.MembershipDomain.
// added and removed contain user IDs (not emails).
func (t *PAPTx) ApplyMemberDeltas(name string, added, removed []string) error {
	subject := domain.GroupResource(name)

	for _, userID := range removed {
		if err := t.txe.RemoveRoleForUser(userID, subject, domain.MembershipDomain); err != nil {
			return fmt.Errorf("remove membership: %w", err)
		}
	}
	for _, userID := range added {
		if err := t.txe.AddRoleForUser(userID, subject, domain.MembershipDomain); err != nil {
			return fmt.Errorf("add membership: %w", err)
		}
	}

	return nil
}

// ApplyUserMembershipDeltas removes the user from the listed groups and then
// adds the user to the new ones. Group names are resolved to subjects
// internally; membership g-rules live on domain.MembershipDomain.
// userID is the User.ID (UUID), not the email.
func (t *PAPTx) ApplyUserMembershipDeltas(userID string, added, removed []string) error {
	for _, name := range removed {
		if err := t.txe.RemoveRoleForUser(userID, domain.GroupResource(name), domain.MembershipDomain); err != nil {
			return fmt.Errorf("remove membership %s: %w", name, err)
		}
	}
	for _, name := range added {
		if err := t.txe.AddRoleForUser(userID, domain.GroupResource(name), domain.MembershipDomain); err != nil {
			return fmt.Errorf("add membership %s: %w", name, err)
		}
	}

	return nil
}

// DeleteGroup removes every Casbin rule attached to the group subject — its
// own permissions (p-rules), its role assignments, and all memberships
// pointing at it.
func (t *PAPTx) DeleteGroup(name string) error {
	if err := t.txe.DeleteUser(domain.GroupResource(name)); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	return nil
}

// DeleteUser removes every Casbin rule referencing the user ID, both as a
// subject (its role assignments / memberships) and as a target (none in the
// current model, but covered for symmetry).
func (t *PAPTx) DeleteUser(userID string) error {
	if err := t.txe.DeleteUser(userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}
