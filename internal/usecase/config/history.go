package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/mock_history.go -package=config_mock . historyEnforcer,configHistoryReader

const defaultHistoryLimit = 20

type historyEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type configHistoryReader interface {
	GetConfigHistory(ctx context.Context, path, namespace string, limit int) ([]*domain.HistoryEntry, error)
	GetAtRevision(ctx context.Context, path, namespace string, revision int64) (*domain.HistoryEntry, error)
}

type HistoryUseCase struct {
	enforcer historyEnforcer
	configs  configHistoryReader
}

func NewHistoryUseCase(enforcer historyEnforcer, configs configHistoryReader) *HistoryUseCase {
	return &HistoryUseCase{enforcer: enforcer, configs: configs}
}

func (uc *HistoryUseCase) GetHistory(
	ctx context.Context,
	path, namespace string,
	limit int,
) ([]*domain.HistoryEntry, error) {
	if namespace == "" {
		return nil, domain.NewValidationError("namespace", "namespace is required")
	}

	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, namespace, "config", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	if limit <= 0 {
		limit = defaultHistoryLimit
	}

	entries, err := uc.configs.GetConfigHistory(ctx, path, namespace, limit)
	if err != nil {
		return nil, fmt.Errorf("get config history: %w", err)
	}

	return entries, nil
}

func (uc *HistoryUseCase) GetAtRevision(
	ctx context.Context,
	path, namespace string,
	revision int64,
) (*domain.HistoryEntry, error) {
	if namespace == "" {
		return nil, domain.NewValidationError("namespace", "namespace is required")
	}

	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := uc.enforcer.Enforce(claims.Email, namespace, "config", "read")
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	entry, err := uc.configs.GetAtRevision(ctx, path, namespace, revision)
	if err != nil {
		return nil, fmt.Errorf("get config at revision: %w", err)
	}

	return entry, nil
}
