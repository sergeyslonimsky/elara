package token

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// Get returns the token if the caller holds (Token, Read) on at least one of
// the token's scoped namespaces. No ownership bypass: tokens are service
// credentials, not user-owned resources.
func (s *Service) Get(ctx context.Context, user domain.AuthInfo, id string) (*domain.Token, error) {
	token, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	for _, ns := range token.Namespaces {
		if s.pdp.Has(user.Email, domain.Permission{
			Object: domain.ObjectToken,
			Action: domain.ActionRead,
			Domain: ns,
		}) {
			return token, nil
		}
	}

	return nil, domain.ErrForbidden
}
