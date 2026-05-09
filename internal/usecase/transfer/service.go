package transfer

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=transfer_mock -source=service.go

type (
	enforcer interface {
		Enforce(subject, domain, object, action string) (bool, error)
	}

	configs interface {
		ListAllByNamespace(ctx context.Context, namespace string) ([]*domain.Config, error)
		Get(ctx context.Context, path, namespace string) (*domain.Config, error)
		Create(ctx context.Context, cfg *domain.Config) error
		Update(ctx context.Context, cfg *domain.Config) error
	}

	namespaces interface {
		List(ctx context.Context) ([]*domain.Namespace, error)
		Get(ctx context.Context, name string) (*domain.Namespace, error)
		Create(ctx context.Context, ns *domain.Namespace) error
	}
)

type Service struct {
	enforcer   enforcer
	configs    configs
	namespaces namespaces
}

func New(enforcer enforcer, configs configs, namespaces namespaces) *Service {
	return &Service{
		enforcer:   enforcer,
		configs:    configs,
		namespaces: namespaces,
	}
}
