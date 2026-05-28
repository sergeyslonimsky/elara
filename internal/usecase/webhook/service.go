package webhook

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=webhook_mock -source=service.go

type (
	pdp interface {
		Has(principal string, perm domain.Permission) bool
		EffectiveDomains(principal string, object domain.Object, action domain.Action) authz.DomainSet
	}

	repo interface {
		Create(ctx context.Context, w *domain.Webhook) error
		Get(ctx context.Context, id string) (*domain.Webhook, error)
		Update(ctx context.Context, w *domain.Webhook) error
		Delete(ctx context.Context, id string) error
		List(ctx context.Context) ([]*domain.Webhook, error)
	}

	dispatcher interface {
		ClearHistory(webhookID string)
		GetDeliveryHistory(webhookID string) []domain.DeliveryAttempt
	}
)

type Service struct {
	pdp        pdp
	repo       repo
	dispatcher dispatcher
}

func New(pdp pdp, repo repo, dispatcher dispatcher) *Service {
	return &Service{
		pdp:        pdp,
		repo:       repo,
		dispatcher: dispatcher,
	}
}
