package user

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=user_mock -source=service.go

// UserReader is the read/write surface the usecase needs from user
// persistence. Like GroupReader, it carries WithTx so the same value can be
// used both inside an open PAP write transaction (returning a tx-scoped
// view) and outside one (mockable in unit tests).
type UserReader interface {
	Get(ctx context.Context, email string) (*domain.User, error)
	List(
		ctx context.Context,
		filter domain.UserFilter,
		params domain.UserListParams,
	) ([]*domain.User, int, error)
	Upsert(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, email string) error
	SetPassword(ctx context.Context, email, hash string, changeRequired bool) error
	SetMembershipVersion(ctx context.Context, email string, version int64) error
	WithTx(tx storage.Tx) UserReader
}

// BoltUserReader adapts *bbolt.UserRepo to UserReader. The adapter is a
// 1:1 method-forwarder; WithTx returns a new adapter scoped to the given tx.
type BoltUserReader struct{ repo *bbolt.UserRepo }

// NewBoltUserReader wraps a bbolt UserRepo so it satisfies UserReader.
func NewBoltUserReader(repo *bbolt.UserRepo) BoltUserReader {
	return BoltUserReader{repo: repo}
}

//nolint:wrapcheck // pure pass-through adapter; usecase wraps at call site.
func (a BoltUserReader) Get(ctx context.Context, email string) (*domain.User, error) {
	return a.repo.Get(ctx, email)
}

//nolint:wrapcheck // pure pass-through adapter; usecase wraps at call site.
func (a BoltUserReader) List(
	ctx context.Context, filter domain.UserFilter, params domain.UserListParams,
) ([]*domain.User, int, error) {
	return a.repo.List(ctx, filter, params)
}

//nolint:wrapcheck // pure pass-through adapter; usecase wraps at call site.
func (a BoltUserReader) Upsert(ctx context.Context, user *domain.User) error {
	return a.repo.Upsert(ctx, user)
}

//nolint:wrapcheck // pure pass-through adapter; usecase wraps at call site.
func (a BoltUserReader) Delete(ctx context.Context, email string) error {
	return a.repo.Delete(ctx, email)
}

//
//nolint:wrapcheck // pure pass-through adapter; usecase wraps at call site.
func (a BoltUserReader) SetPassword(
	ctx context.Context,
	email, hash string,
	changeRequired bool,
) error {
	return a.repo.SetPassword(ctx, email, hash, changeRequired)
}

//
//nolint:wrapcheck // pure pass-through adapter; usecase wraps at call site.
func (a BoltUserReader) SetMembershipVersion(
	ctx context.Context,
	email string,
	version int64,
) error {
	return a.repo.SetMembershipVersion(ctx, email, version)
}

//nolint:ireturn // interface return is required by the UserReader contract.
func (a BoltUserReader) WithTx(tx storage.Tx) UserReader {
	return BoltUserReader{repo: a.repo.WithTx(tx)}
}

// GroupReader is the narrow read-only port the usecase needs over the group
// repo. Same pattern as UserReader: WithTx for tx-scoped views, interface
// for mockability. Exported because mockgen lives in a sibling package.
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

//nolint:wrapcheck // pure pass-through adapter; usecase wraps at call site.
func (a BoltGroupReader) Get(ctx context.Context, id string) (*domain.Group, error) {
	return a.repo.Get(ctx, id)
}

//nolint:wrapcheck // pure pass-through adapter; usecase wraps at call site.
func (a BoltGroupReader) FindByName(ctx context.Context, name string) (*domain.Group, error) {
	return a.repo.FindByName(ctx, name)
}

//nolint:ireturn // interface return is required by the GroupReader contract.
func (a BoltGroupReader) WithTx(tx storage.Tx) GroupReader {
	return BoltGroupReader{repo: a.repo.WithTx(tx)}
}

type Service struct {
	store  UserReader
	groups GroupReader
	pdp    *authz.PDP
	pap    *authz.PAP
	scope  *authz.Scope
}

func New(
	store UserReader,
	groups GroupReader,
	pdp *authz.PDP,
	pap *authz.PAP,
	scope *authz.Scope,
) *Service {
	return &Service{
		store:  store,
		groups: groups,
		pdp:    pdp,
		pap:    pap,
		scope:  scope,
	}
}
