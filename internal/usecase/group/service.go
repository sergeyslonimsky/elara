package group

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=group_mock -source=service.go

type (
	enforcer interface {
		Enforce(subject, domain, object, action string) (bool, error)
	}

	syncEnforcer interface {
		AddRoleForUser(user, role, domain string) error
		RemoveRoleForUser(user, role, domain string) error
		GetRulesForSubject(subject string) [][]string
	}

	store interface {
		Create(ctx context.Context, group *domain.Group) error
		Get(ctx context.Context, id string) (*domain.Group, error)
		Update(ctx context.Context, group *domain.Group) error
		Delete(ctx context.Context, id string) error
		List(ctx context.Context) ([]*domain.Group, error)
		FindByName(ctx context.Context, name string) (*domain.Group, error)
	}
)

type Service struct {
	enforcer     enforcer
	syncEnforcer syncEnforcer
	store        store
}

func New(enforcer enforcer, syncEnforcer syncEnforcer, store store) *Service {
	return &Service{
		enforcer:     enforcer,
		syncEnforcer: syncEnforcer,
		store:        store,
	}
}

const (
	errGetGroup    = "get group: %w"
	errUpdateGroup = "update group: %w"
)
