package group

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// Service orchestrates group lifecycle and Casbin role/membership rules.
//
// All mutating methods route through Enforcer.WriteTx so the bbolt write of
// the Group entity and the Casbin g-rules update commit (or roll back)
// atomically — preserving the §4 level-2 invariant from EL-4.
//
// store and enforcer are concrete pointers rather than interfaces because
// the per-tx views (bbolt.GroupRepo.WithTx, casbin.Enforcer.WithTx) return
// concrete types. Tests use a real bbolt + real Enforcer integration helper
// instead of mocks (see service_test.go).
type Service struct {
	store *bbolt.GroupRepo
	txm   storage.TxManager
	pdp   *authz.PDP
	pap   *authz.PAP
}

func New(
	store *bbolt.GroupRepo,
	txm storage.TxManager,
	pdp *authz.PDP,
	pap *authz.PAP,
) *Service {
	return &Service{
		store: store,
		txm:   txm,
		pdp:   pdp,
		pap:   pap,
	}
}

const (
	errGetGroup    = "get group: %w"
	errUpdateGroup = "update group: %w"
)

const defaultListLimit = 20

// loadGroupForUpdate fetches a group via the given tx, rejects system
// groups via EnsureMutable, then verifies the optimistic-concurrency
// version. Used by Update.
func (s *Service) loadGroupForUpdate(
	ctx context.Context,
	tx storage.Tx,
	id string,
	version int64,
) (*domain.Group, error) {
	existing, err := s.loadMutableGroup(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	if existing.Version != version {
		return nil, domain.ErrVersionConflict
	}

	return existing, nil
}

// loadMutableGroup fetches a group within the given tx and rejects system
// groups via EnsureMutable. Shared by Update and Delete.
func (s *Service) loadMutableGroup(
	ctx context.Context,
	tx storage.Tx,
	id string,
) (*domain.Group, error) {
	existing, err := s.store.WithTx(tx).Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	if err := existing.EnsureMutable(); err != nil {
		return nil, fmt.Errorf("ensure mutable: %w", err)
	}

	return existing, nil
}
