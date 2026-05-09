package clients

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

// SubscribeChanges checks authorization and returns the change channel.
func (s *Service) SubscribeChanges(ctx context.Context) (<-chan domain.ClientChange, func(), error) {
	if err := auth.CheckAccess(ctx, s.enforcer, auth.ObjectAll, auth.ObjectClient, auth.ActionRead); err != nil {
		return nil, nil, fmt.Errorf("check access: %w", err)
	}

	ch, cancel := s.active.Subscribe()

	return ch, cancel, nil
}

// SubscribeClient checks authorization and returns the per-client change channel.
func (s *Service) SubscribeClient(ctx context.Context, connID string) (<-chan domain.ClientChange, func(), error) {
	if err := auth.CheckAccess(ctx, s.enforcer, auth.ObjectAll, auth.ObjectClient, auth.ActionRead); err != nil {
		return nil, nil, fmt.Errorf("check access: %w", err)
	}

	ch, cancel := s.active.SubscribeClient(connID)

	return ch, cancel, nil
}
