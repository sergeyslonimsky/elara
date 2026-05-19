package profile

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type MeResult struct {
	Email       string
	Name        string
	Permissions []domain.Permission
}

func (s *Service) Me(ctx context.Context) (*MeResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	permissions, err := s.pdp.ListPermissions(claims.Email)
	if err != nil {
		return nil, fmt.Errorf("me: %w", err)
	}

	return &MeResult{
		Email:       claims.Email,
		Name:        claims.Name,
		Permissions: permissions,
	}, nil
}
