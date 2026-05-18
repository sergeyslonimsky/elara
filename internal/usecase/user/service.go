package user

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/adapter/bbolt"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
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
	List(ctx context.Context) ([]*domain.User, error)
	Upsert(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, email string) error
	SetPassword(ctx context.Context, email, hash string, changeRequired bool) error
}

type Service struct {
	enforcer *casbin.Enforcer
	store    store
	users    *bbolt.UserRepo
	txm      storage.TxManager
}

func New(enforcer *casbin.Enforcer, store store, users *bbolt.UserRepo, txm storage.TxManager) *Service {
	return &Service{
		enforcer: enforcer,
		store:    store,
		users:    users,
		txm:      txm,
	}
}
