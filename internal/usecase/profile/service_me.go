package profile

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type MeResult struct {
	Email       string
	Name        string
	Permissions []domain.Permission
}

func (s *Service) Me(ctx context.Context) (*MeResult, error) {
	user, ok := authctx.UserFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	permissions, err := s.pdp.ListPermissions(user.ID.String())
	if err != nil {
		return nil, fmt.Errorf("me: %w", err)
	}

	return &MeResult{
		Email:       user.Email,
		Name:        user.DisplayName,
		Permissions: permissions,
	}, nil
}
