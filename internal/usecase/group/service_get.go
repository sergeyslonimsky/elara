package group

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func (s *Service) Get(ctx context.Context, id string) (*domain.Group, error) {
	group, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errGetGroup, err)
	}

	return group, nil
}
