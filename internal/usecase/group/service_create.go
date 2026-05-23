package group

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func (s *Service) Create(ctx context.Context, _ domain.AuthInfo, name string) (*domain.Group, error) {
	now := time.Now().UTC()
	group := &domain.Group{
		ID:        uuid.New().String(),
		Name:      name,
		Members:   []string{}, // fresh group; no Casbin g-rules yet
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := group.Validate(); err != nil {
		return nil, fmt.Errorf("validate group: %w", err)
	}

	if err := s.store.Create(ctx, group); err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}

	return group, nil
}
