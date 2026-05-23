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
