package token

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Revoke deletes the token if the caller holds (Token, Write) on at least one
// of the token's scoped namespaces. No ownership bypass.
func (s *Service) Revoke(ctx context.Context, user domain.AuthInfo, id string) error {
	token, err := s.store.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get token for revocation: %w", err)
	}

	allowed := false

	for _, ns := range token.Namespaces {
		if s.pdp.Has(user.UserID, domain.Permission{
			Object: domain.ObjectToken,
			Action: domain.ActionWrite,
			Domain: ns,
		}) {
			allowed = true

			break
		}
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	return nil
}
