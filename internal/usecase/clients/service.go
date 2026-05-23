package clients

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=clients_mock -source=service.go

// Historical lookup. We don't have an indexed Get-by-ID in the history
// repo, so we scan the most recent N. For typical UI usage (operator
// clicked through a recent disconnection) this is fast enough.
const historyScanLimit = 1000

type (
	pdp interface {
		Has(principal string, perm domain.Permission) bool
	}

	activeSource interface {
		ListActive() []*domain.Client
		Get(connID string) *domain.Client
		RecentEvents(connID string) []domain.ClientEvent
		Subscribe() (<-chan domain.ClientChange, func())
		SubscribeClient(connID string) (<-chan domain.ClientChange, func())
	}

	historySource interface {
		List(ctx context.Context, limit int) ([]*domain.Client, error)
		ListByClient(ctx context.Context, clientName, k8sNamespace string, limit int) ([]*domain.Client, error)
	}
)

type Service struct {
	pdp     pdp
	active  activeSource
	history historySource
}

func New(pdp pdp, active activeSource, history historySource) *Service {
	return &Service{pdp: pdp, active: active, history: history}
}
