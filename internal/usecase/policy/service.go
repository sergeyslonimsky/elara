package policy

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=policy_mock -source=service.go

type (
	enforcer interface {
		Enforce(subject, domain, object, action string) (bool, error)
		AddRoleForUser(user, role, domain string) error
		RemoveRoleForUser(user, role, domain string) error
		GetGroupingPolicy() [][]string
	}

	groupFinder interface {
		FindByName(ctx context.Context, name string) (*domain.Group, error)
	}
)

type Service struct {
	enforcer enforcer
	groups   groupFinder
}

func New(enforcer enforcer, groups groupFinder) *Service {
	return &Service{
		enforcer: enforcer,
		groups:   groups,
	}
}
