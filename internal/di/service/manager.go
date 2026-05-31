package service

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
)

type Managers struct {
	Adapters     *Adapters
	Services     *Services
	V2Handlers   *V2Handlers
	EtcdHandlers *EtcdHandlers

	// Plural aliases for plural naming consistency
	Enforcer *casbin.Enforcer
	Sessions *sessions.Service
}

func NewServiceManager(
	ctx context.Context,
	cfg config.Config,
) (*Managers, func(context.Context) error, error) {
	adapters, err := NewAdapters(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create adapters: %w", err)
	}

	enforcer, err := casbin.NewEnforcer(adapters.AuthPolicy)
	if err != nil {
		_ = adapters.Shutdown(ctx)

		return nil, nil, fmt.Errorf("create enforcer: %w", err)
	}

	sessionSvc := newSessionService(adapters)

	services, err := NewServices(ctx, adapters, cfg, enforcer, sessionSvc)
	if err != nil {
		_ = adapters.Shutdown(ctx)

		return nil, nil, fmt.Errorf("create services: %w", err)
	}

	handlers := NewV2Handlers(services, sessionSvc, cfg)
	etcdHandlers := NewEtcdHandlers(adapters)

	mgrs := &Managers{
		Adapters:     adapters,
		Services:     services,
		V2Handlers:   handlers,
		EtcdHandlers: etcdHandlers,
		Enforcer:     enforcer,
		Sessions:     sessionSvc,
	}

	cleanup := func(ctx context.Context) error {
		return adapters.Shutdown(ctx)
	}

	return mgrs, cleanup, nil
}

// NewManagers is a legacy-compatibility helper for NewServiceManager.
func NewManagers(
	ctx context.Context,
	cfg config.Config,
	a *Adapters,
) (*Managers, error) {
	enforcer, err := casbin.NewEnforcer(a.AuthPolicy)
	if err != nil {
		return nil, fmt.Errorf("create enforcer: %w", err)
	}

	return &Managers{
		Enforcer: enforcer,
		Sessions: newSessionService(a),
	}, nil
}

// newSessionService wires the bbolt session/event repos and the txm-backed
// sessions.Service.
func newSessionService(a *Adapters) *sessions.Service {
	return sessions.New(
		a.SessionRepo,
		a.SessionEventRepo,
		sessions.RealClock{},
	)
}
