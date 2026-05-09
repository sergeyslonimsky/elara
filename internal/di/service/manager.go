package service

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
)

type Manager struct {
	Adapters       *Adapters
	Services       *Services
	V2Handlers     *V2Handlers
	EtcdHandlers   *EtcdHandlers
	SessionManager *auth.SessionManager
	Enforcer       *casbin.Enforcer
}

// NewServiceManager wires the full service tree. On partial failure the
// returned cleanup closure tears down whatever was already opened — runtime
// teardown on success is driven separately by app.App.
func NewServiceManager(
	ctx context.Context,
	cfg config.Config,
) (*Manager, func(context.Context) error, error) {
	adapters, err := NewAdapters(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("init adapters: %w", err)
	}

	cleanup := func(ctx context.Context) error {
		return adapters.Shutdown(ctx)
	}

	enforcer, err := casbin.NewEnforcer(adapters.AuthPolicy)
	if err != nil {
		return nil, cleanup, fmt.Errorf("create casbin enforcer: %w", err)
	}

	sessionManager := auth.NewSessionManager(cfg.UI.Auth.Session.Secret, cfg.UI.Auth.Session.TTL)

	services, err := NewServices(ctx, adapters, cfg, enforcer, sessionManager)
	if err != nil {
		return nil, cleanup, fmt.Errorf("init services: %w", err)
	}

	return &Manager{
		Adapters:       adapters,
		Services:       services,
		V2Handlers:     NewV2Handlers(services, cfg),
		EtcdHandlers:   NewEtcdHandlers(adapters),
		SessionManager: sessionManager,
		Enforcer:       enforcer,
	}, cleanup, nil
}
