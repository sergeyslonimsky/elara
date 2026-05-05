package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_history.go -package=webhook_mock . deliveryHistoryProvider,historyWebhookGetter

type historyEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type deliveryHistoryProvider interface {
	GetDeliveryHistory(webhookID string) []domain.DeliveryAttempt
}

type historyWebhookGetter interface {
	Get(ctx context.Context, id string) (*domain.Webhook, error)
}

type HistoryUseCase struct {
	enforcer   historyEnforcer
	dispatcher deliveryHistoryProvider
	getter     historyWebhookGetter
}

func NewHistoryUseCase(
	enforcer historyEnforcer,
	dispatcher deliveryHistoryProvider,
	getter historyWebhookGetter,
) *HistoryUseCase {
	return &HistoryUseCase{enforcer: enforcer, dispatcher: dispatcher, getter: getter}
}

func (uc *HistoryUseCase) Execute(ctx context.Context, webhookID string) ([]domain.DeliveryAttempt, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	w, err := uc.getter.Get(ctx, webhookID)
	if err != nil {
		return nil, fmt.Errorf("get webhook: %w", err)
	}

	ns := w.NamespaceFilter
	if ns == "" {
		ns = "*"
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, ns, "webhook", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	return uc.dispatcher.GetDeliveryHistory(webhookID), nil
}
