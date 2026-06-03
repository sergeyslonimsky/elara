package webhook

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/storage"
)

// Create persists a new webhook. The handler is responsible for the
// (Webhook, Create, namespace) authorization gate before this is called.
func (s *Service) Create(ctx context.Context, w *domain.Webhook) (*domain.Webhook, error) {
	if err := w.Validate(); err != nil {
		return nil, fmt.Errorf("validate webhook: %w", err)
	}

	if w.ID == "" {
		w.ID = uuid.New().String()
	}

	now := time.Now()
	w.CreatedAt = now
	w.UpdatedAt = now

	if err := s.repo.Create(ctx, w); err != nil {
		if errors.Is(err, storage.ErrResourceAlreadyExists) {
			return nil, fmt.Errorf("create webhook: %w", domain.ErrAlreadyExists)
		}

		return nil, fmt.Errorf("create webhook: %w", err)
	}

	return w, nil
}
