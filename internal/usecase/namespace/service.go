package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=namespace_mock -source=service.go

type (
	pdp interface {
		Has(principal string, perm domain.Permission) bool
		EffectiveDomains(
			principal string,
			object domain.Object,
			action domain.Action,
		) authz.DomainSet
	}

	store interface {
		Create(ctx context.Context, ns *domain.Namespace) error
		Delete(ctx context.Context, name string) error
		Get(ctx context.Context, name string) (*domain.Namespace, error)
		List(
			ctx context.Context,
			filter domain.NamespaceFilter,
			params domain.NamespaceListParams,
		) ([]*domain.Namespace, int, error)
		Update(ctx context.Context, ns *domain.Namespace) error
		LockNamespace(ctx context.Context, name string) error
		UnlockNamespace(ctx context.Context, name string) error
		CountConfigs(ctx context.Context, name string) (int, error)
	}

	notifier interface {
		NotifyNamespaceLocked(ctx context.Context, namespace string)
		NotifyNamespaceUnlocked(ctx context.Context, namespace string)
	}
)

type Service struct {
	pdp      pdp
	store    store
	notifier notifier
}

func New(pdp pdp, store store, notifier notifier) *Service {
	return &Service{
		pdp:      pdp,
		store:    store,
		notifier: notifier,
	}
}

func (s *Service) populateConfigCount(ctx context.Context, ns *domain.Namespace) error {
	count, err := s.store.CountConfigs(ctx, ns.Name)
	if err != nil {
		return fmt.Errorf("count configs: %w", err)
	}

	ns.ConfigCount = count

	return nil
}

func (s *Service) populateConfigCounts(ctx context.Context, namespaces []*domain.Namespace) error {
	for _, ns := range namespaces {
		if err := s.populateConfigCount(ctx, ns); err != nil {
			return err
		}
	}

	return nil
}
