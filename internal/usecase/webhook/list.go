package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_list.go -package=webhook_mock . webhookLister

type listEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type webhookLister interface {
	List(ctx context.Context) ([]*domain.Webhook, error)
}

type ListUseCase struct {
	enforcer listEnforcer
	repo     webhookLister
}

func NewListUseCase(enforcer listEnforcer, repo webhookLister) *ListUseCase {
	return &ListUseCase{enforcer: enforcer, repo: repo}
}

func (uc *ListUseCase) Execute(ctx context.Context) ([]*domain.Webhook, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	webhooks, err := uc.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}

	// Filter silently: only return webhooks the caller can read.
	filtered := webhooks[:0]
	for _, w := range webhooks {
		ns := w.NamespaceFilter
		if ns == "" {
			ns = "*"
		}

		allowed, _ := uc.enforcer.Enforce(claims.Email, ns, "webhook", "read")
		if allowed {
			filtered = append(filtered, w)
		}
	}

	return filtered, nil
}
