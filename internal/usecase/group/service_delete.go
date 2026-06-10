package group

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
)

func (s *Service) Delete(ctx context.Context, _ domain.AuthInfo, name string) error {
	err := s.pap.Write(ctx, func(ctx context.Context, w *authz.PAPTx) error {
		group, err := s.loadMutableGroup(ctx, name)
		if err != nil {
			return err
		}

		if err := w.DeleteGroup(group.Name); err != nil {
			return fmt.Errorf("pap delete group: %w", err)
		}

		if err := s.store.Delete(ctx, name); err != nil {
			return fmt.Errorf("delete group: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("delete group transaction: %w", err)
	}

	return nil
}
