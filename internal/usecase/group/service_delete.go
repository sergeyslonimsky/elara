package group

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

func (s *Service) Delete(ctx context.Context, _ domain.AuthInfo, id string) error {
	err := s.pap.Write(ctx, func(tx storage.Tx, w *authz.PAPTx) error {
		group, err := s.loadMutableGroup(ctx, tx, id)
		if err != nil {
			return err
		}

		if err := w.DeleteGroup(group.Name); err != nil {
			return err
		}

		if err := s.store.WithTx(tx).Delete(ctx, id); err != nil {
			return fmt.Errorf("delete group: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("delete group transaction: %w", err)
	}

	return nil
}
