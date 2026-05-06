package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_watch.go -package=config_mock . watchEnforcer,watchSubscriber

type watchEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type watchSubscriber interface {
	Subscribe(ctx context.Context, pathPrefix, namespace string) (<-chan domain.WatchEvent, func())
}

type WatchUseCase struct {
	enforcer watchEnforcer
	watch    watchSubscriber
}

func NewWatchUseCase(enforcer watchEnforcer, watch watchSubscriber) *WatchUseCase {
	return &WatchUseCase{enforcer: enforcer, watch: watch}
}

func (uc *WatchUseCase) Execute(
	ctx context.Context,
	pathPrefix, namespace string,
) (<-chan domain.WatchEvent, func(), error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, nil, domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, namespace, "config", "read")
	if err != nil {
		return nil, nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, nil, domain.ErrForbidden
	}

	if pathPrefix == "" {
		pathPrefix = "/"
	}

	ch, cancel := uc.watch.Subscribe(ctx, pathPrefix, namespace)

	return ch, cancel, nil
}
