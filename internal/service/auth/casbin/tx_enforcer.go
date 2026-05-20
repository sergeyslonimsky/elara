package casbin

import (
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
)

// opKind enumerates the casbin model mutations recorded inside a transaction
// so they can be replayed against the in-memory cache after the underlying
// bbolt tx commits.
type opKind int

const (
	opAddP opKind = iota
	opRemoveP
	opAddG
	opRemoveG
	opDeleteUser
)

// op is a recorded model mutation. For p/g rules `args` holds the full rule
// (sub, dom, obj, act) or (user, role, domain). For opDeleteUser, only `user`
// is set.
type op struct {
	kind opKind
	args []string
	user string
}

// TxEnforcer is a per-transaction view of Enforcer. All mutation methods
// write rules directly through PolicyRepo.WithTx(tx) and record the operation
// for post-commit cache sync. Returned by Enforcer.WithTx.
type TxEnforcer struct {
	parent   *Enforcer
	policies *bbolt.PolicyRepo
	ops      []op
}

// AddPolicy persists a p-rule inside the bound transaction.
func (t *TxEnforcer) AddPolicy(sub, dom, obj, act string) error {
	rule := []string{sub, dom, obj, act}
	if err := t.policies.AddPolicy("p", "p", rule); err != nil {
		return fmt.Errorf("tx add policy: %w", err)
	}

	t.ops = append(t.ops, op{kind: opAddP, args: rule})

	return nil
}

// RemovePolicy removes a p-rule inside the bound transaction.
func (t *TxEnforcer) RemovePolicy(sub, dom, obj, act string) error {
	rule := []string{sub, dom, obj, act}
	if err := t.policies.RemovePolicy("p", "p", rule); err != nil {
		return fmt.Errorf("tx remove policy: %w", err)
	}

	t.ops = append(t.ops, op{kind: opRemoveP, args: rule})

	return nil
}

// AddRoleForUser persists a g-rule (role assignment) inside the bound transaction.
func (t *TxEnforcer) AddRoleForUser(user, role, dom string) error {
	rule := []string{user, role, dom}
	if err := t.policies.AddPolicy("g", "g", rule); err != nil {
		return fmt.Errorf("tx add role for user: %w", err)
	}

	t.ops = append(t.ops, op{kind: opAddG, args: rule})

	return nil
}

// RemoveRoleForUser removes a g-rule inside the bound transaction.
func (t *TxEnforcer) RemoveRoleForUser(user, role, dom string) error {
	rule := []string{user, role, dom}
	if err := t.policies.RemovePolicy("g", "g", rule); err != nil {
		return fmt.Errorf("tx remove role for user: %w", err)
	}

	t.ops = append(t.ops, op{kind: opRemoveG, args: rule})

	return nil
}

// DeleteUser removes every g-rule where email appears as either the subject
// (column 0, i.e. user→group/role bindings) or the role/group target
// (column 1, i.e. inbound references). Two RemoveFilteredPolicy calls are
// required because casbin's bbolt adapter filters by a single field index at a
// time, and a user identifier can occur in both positions across different
// rule shapes.
func (t *TxEnforcer) DeleteUser(email string) error {
	if err := t.policies.RemoveFilteredPolicy("g", "g", 0, email); err != nil {
		return fmt.Errorf("tx delete user (subject): %w", err)
	}

	if err := t.policies.RemoveFilteredPolicy("g", "g", 1, email); err != nil {
		return fmt.Errorf("tx delete user (role): %w", err)
	}

	t.ops = append(t.ops, op{kind: opDeleteUser, user: email})

	return nil
}

// GetPermissionsForSubject retrieves permissions for the given subject directly from the tx.
func (t *TxEnforcer) GetPermissionsForSubject(subject string) ([][]string, error) {
	rules, err := t.policies.ListPermissionsForSubject(subject)
	if err != nil {
		return nil, fmt.Errorf("list permissions for subject: %w", err)
	}

	return rules, nil
}
