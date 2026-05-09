package user

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=user_mock -source=service.go

type (
	enforcer interface {
		Enforce(subject, domain, object, action string) (bool, error)
		GetGroupingPolicy() [][]string
		DeleteUser(email string) error
	}

	store interface {
		Get(ctx context.Context, email string) (*domain.User, error)
		List(ctx context.Context) ([]*domain.User, error)
		Upsert(ctx context.Context, user *domain.User) error
		Delete(ctx context.Context, email string) error
		SetPassword(ctx context.Context, email, hash string, changeRequired bool) error
	}
)

type Service struct {
	enforcer enforcer
	store    store
}

func New(enforcer enforcer, store store) *Service {
	return &Service{
		enforcer: enforcer,
		store:    store,
	}
}
