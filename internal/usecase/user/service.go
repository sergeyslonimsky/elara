package user

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=user_mock -source=service.go

// store is the read/write surface the usecase needs from the persistence
// layer. Tests substitute a mock for the non-transactional paths
// (Create/List/Get/SetPassword); the Delete path runs through
// casbin.Enforcer.WriteTx with a per-tx UserRepo view (concrete type), so it
// is exercised via the real bbolt + casbin integration helper.
type store interface {
	Get(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context, filter domain.UserFilter, params domain.UserListParams) ([]*domain.User, int, error)
	Upsert(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, email string) error
	SetPassword(ctx context.Context, email, hash string, changeRequired bool) error
}

// GroupReader is the narrow read-only port the usecase needs over the group
// repo. Keeping it an interface (rather than depending on *bbolt.GroupRepo
// directly) lets transactional callers swap the tx-scoped view in via
// WithTx, and lets pure authz-failure tests substitute a stub instead of
// spinning up a real bbolt stack. Exported because mockgen lives in a
// sibling package.
type GroupReader interface {
	Get(ctx context.Context, id string) (*domain.Group, error)
	FindByName(ctx context.Context, name string) (*domain.Group, error)
	WithTx(tx storage.Tx) GroupReader
}

// BoltGroupReader adapts *bbolt.GroupRepo to GroupReader. The adapter is a
// 1:1 method-forwarder; WithTx returns a new adapter scoped to the given tx
// so all reads inside Enforcer.WriteTx see the in-flight snapshot.
type BoltGroupReader struct{ repo *bbolt.GroupRepo }

// NewBoltGroupReader wraps a bbolt GroupRepo so it satisfies GroupReader.
func NewBoltGroupReader(repo *bbolt.GroupRepo) BoltGroupReader {
	return BoltGroupReader{repo: repo}
}

// Get returns the group identified by id.
//
//nolint:wrapcheck // pure pass-through adapter; usecase wraps at call site.
func (a BoltGroupReader) Get(ctx context.Context, id string) (*domain.Group, error) {
	return a.repo.Get(ctx, id)
}

// FindByName returns the group with the given name.
//
//nolint:wrapcheck // pure pass-through adapter; usecase wraps at call site.
func (a BoltGroupReader) FindByName(ctx context.Context, name string) (*domain.Group, error) {
	return a.repo.FindByName(ctx, name)
}

// WithTx returns a reader scoped to the given storage transaction.
//
//nolint:ireturn // interface return is required by the GroupReader contract.
func (a BoltGroupReader) WithTx(tx storage.Tx) GroupReader {
	return BoltGroupReader{repo: a.repo.WithTx(tx)}
}

type Service struct {
	enforcer *casbin.Enforcer
	store    store
	users    *bbolt.UserRepo
	groups   GroupReader
	txm      storage.TxManager
	pdp      *authz.PDP
	pap      *authz.PAP
}

func New(
	enforcer *casbin.Enforcer,
	store store,
	users *bbolt.UserRepo,
	groups GroupReader,
	txm storage.TxManager,
	pdp *authz.PDP,
	pap *authz.PAP,
) *Service {
	return &Service{
		enforcer: enforcer,
		store:    store,
		users:    users,
		groups:   groups,
		txm:      txm,
		pdp:      pdp,
		pap:      pap,
	}
}
