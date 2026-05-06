package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_get.go -package=webhook_mock . webhookGetter

type getEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type webhookGetter interface {
	Get(ctx context.Context, id string) (*domain.Webhook, error)
}

type GetUseCase struct {
	enforcer getEnforcer
	repo     webhookGetter
}

func NewGetUseCase(enforcer getEnforcer, repo webhookGetter) *GetUseCase {
	return &GetUseCase{enforcer: enforcer, repo: repo}
}

func (uc *GetUseCase) Execute(ctx context.Context, id string) (*domain.Webhook, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	// Fetch first to know the namespace filter, then enforce.
	w, err := uc.repo.Get(ctx, id)
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

	return w, nil
}
