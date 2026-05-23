package webhook

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Create persists a new webhook. The handler is responsible for the
// (Webhook, Create, namespace) authorization gate before this is called.
func (s *Service) Create(ctx context.Context, w *domain.Webhook) (*domain.Webhook, error) {
	if err := w.Validate(); err != nil {
		return nil, fmt.Errorf("validate webhook: %w", err)
	}

	if err := s.repo.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("create webhook: %w", err)
	}

	return w, nil
}
