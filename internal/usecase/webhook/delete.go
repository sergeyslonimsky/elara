package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_delete.go -package=webhook_mock . deleteEnforcer,deleteWebhookGetter,webhookDeleter,historyClearer

type deleteEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type deleteWebhookGetter interface {
	Get(ctx context.Context, id string) (*domain.Webhook, error)
}

type webhookDeleter interface {
	Delete(ctx context.Context, id string) error
}

type historyClearer interface {
	ClearHistory(webhookID string)
}

type DeleteUseCase struct {
	enforcer   deleteEnforcer
	getter     deleteWebhookGetter
	repo       webhookDeleter
	dispatcher historyClearer
}

func NewDeleteUseCase(
	enforcer deleteEnforcer,
	getter deleteWebhookGetter,
	repo webhookDeleter,
	dispatcher historyClearer,
) *DeleteUseCase {
	return &DeleteUseCase{enforcer: enforcer, getter: getter, repo: repo, dispatcher: dispatcher}
}

func (uc *DeleteUseCase) Execute(ctx context.Context, id string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	w, err := uc.getter.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get webhook: %w", err)
	}

	ns := w.NamespaceFilter
	if ns == "" {
		ns = "*"
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, ns, "webhook", "write")
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := uc.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}

	uc.dispatcher.ClearHistory(id)

	return nil
}
