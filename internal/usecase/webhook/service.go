package webhook

import (
	"context"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/service_mock.go -package=webhook_mock -source=service.go

type (
	enforcer interface {
		Enforce(subject, domain, object, action string) (bool, error)
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
	enforcer   enforcer
	repo       repo
	dispatcher dispatcher
}

func New(enforcer enforcer, repo repo, dispatcher dispatcher) *Service {
	return &Service{
		enforcer:   enforcer,
		repo:       repo,
		dispatcher: dispatcher,
	}
}
