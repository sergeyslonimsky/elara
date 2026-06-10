package namespace

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=namespace_mock -source=service.go

type (
	pdp interface {
		Has(principal string, perm domain.Permission) bool
		HasNamespace(actor, name string, action domain.Action) bool
		EffectiveDomains(
			principal string,
			object domain.Object,
			action domain.Action,
		) authz.DomainSet
		EffectiveNamespaces(actor string, action domain.Action) authz.DomainSet
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
	}

	configCounter interface {
		CountByNamespace(ctx context.Context, namespace string) (int, error)
	}

	notifier interface {
		NotifyNamespaceLocked(ctx context.Context, namespace string)
		NotifyNamespaceUnlocked(ctx context.Context, namespace string)
	}
)

type Service struct {
	txm      storage.Manager
	pdp      pdp
	store    store
	configs  configCounter
	notifier notifier
}

func New(
	txm storage.Manager,
	pdp pdp,
	store store,
	configs configCounter,
	notifier notifier,
) *Service {
	return &Service{
		txm:      txm,
		pdp:      pdp,
		store:    store,
		configs:  configs,
		notifier: notifier,
	}
}

// populateConfigCount fills ns.ConfigCount via the bbolt cursor in
// configs.CountByNamespace. That cursor requires a tx-backed querier, so we
// run the count inside Manager.WithTx — when this helper is invoked from a
// usecase that already opened a transaction (e.g. service_update) the
// inner WithTx flattens onto the outer one and reuses its tx.
func (s *Service) populateConfigCount(ctx context.Context, ns *domain.Namespace) error {
	if err := s.txm.WithTx(ctx, func(ctx context.Context) error {
		count, err := s.configs.CountByNamespace(ctx, ns.Name)
		if err != nil {
			return fmt.Errorf("count configs: %w", err)
		}

		ns.ConfigCount = count

		return nil
	}); err != nil {
		return fmt.Errorf("populate config count: %w", err)
	}

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
