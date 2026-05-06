package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_create.go -package=webhook_mock . webhookCreator

type createEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type webhookCreator interface {
	Create(ctx context.Context, w *domain.Webhook) error
}

type CreateUseCase struct {
	enforcer createEnforcer
	repo     webhookCreator
}

func NewCreateUseCase(enforcer createEnforcer, repo webhookCreator) *CreateUseCase {
	return &CreateUseCase{enforcer: enforcer, repo: repo}
}

func (uc *CreateUseCase) Execute(ctx context.Context, w *domain.Webhook) (*domain.Webhook, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	ns := w.NamespaceFilter
	if ns == "" {
		ns = "*"
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, ns, "webhook", "write")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	if err := w.Validate(); err != nil {
		return nil, fmt.Errorf("validate webhook: %w", err)
	}

	if err := uc.repo.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("create webhook: %w", err)
	}

	return w, nil
}
