package policy

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=policy_mock -source=service.go

// groupFinder lets us reject AssignRole/RevokeRole for groups that don't
// exist. Without this check it would be possible to grant a role to a
// typo'd group name — the g-rule would sit in Casbin orphaned, with no UI
// surface to discover or revoke it.
type groupFinder interface {
	FindByName(ctx context.Context, name string) (*domain.Group, error)
}

// Service grants and revokes group->role assignments through atomic Casbin
// writes (Enforcer.WriteTx). enforcer is a concrete pointer because WriteTx
// hands out a *TxEnforcer; tests use a real Enforcer + bbolt rather than mocks.
type Service struct {
	enforcer *casbin.Enforcer
	groups   groupFinder
	txm      storage.TxManager
}

func New(enforcer *casbin.Enforcer, groups groupFinder, txm storage.TxManager) *Service {
	return &Service{
		enforcer: enforcer,
		groups:   groups,
		txm:      txm,
	}
}
