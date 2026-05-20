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

type Service struct {
	enforcer *casbin.Enforcer
	store    store
	users    *bbolt.UserRepo
	txm      storage.TxManager
	pdp      *authz.PDP
}

func New(
	enforcer *casbin.Enforcer,
	store store,
	users *bbolt.UserRepo,
	txm storage.TxManager,
	pdp *authz.PDP,
) *Service {
	return &Service{
		enforcer: enforcer,
		store:    store,
		users:    users,
		txm:      txm,
		pdp:      pdp,
	}
}
