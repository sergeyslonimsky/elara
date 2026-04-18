package config

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const defaultHistoryLimit = 20

type configHistoryReader interface {
	GetConfigHistory(ctx context.Context, path, namespace string, limit int) ([]*domain.HistoryEntry, error)
	GetAtRevision(ctx context.Context, path, namespace string, revision int64) (*domain.HistoryEntry, error)
}

type HistoryUseCase struct {
	configs configHistoryReader
}

func NewHistoryUseCase(configs configHistoryReader) *HistoryUseCase {
	return &HistoryUseCase{configs: configs}
}

func (uc *HistoryUseCase) GetHistory(
	ctx context.Context,
	path, namespace string,
	limit int,
) ([]*domain.HistoryEntry, error) {
	if namespace == "" {
		namespace = domain.DefaultNamespace
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
		namespace = domain.DefaultNamespace
	}

	entry, err := uc.configs.GetAtRevision(ctx, path, namespace, revision)
	if err != nil {
		return nil, fmt.Errorf("get config at revision: %w", err)
	}

	return entry, nil
}
