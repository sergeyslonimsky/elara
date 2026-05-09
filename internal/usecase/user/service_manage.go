package user

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

func (s *Service) Create(ctx context.Context, email, name, initialPassword string) (*domain.User, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(
		claims.Email,
		auth.ObjectAll,
		auth.ObjectUser,
		auth.ActionWrite,
	)
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	user := &domain.User{
		Email:    email,
		Name:     name,
		Provider: domain.ProviderBasicAuth,
	}

	if initialPassword == "" {
		user.Provider = domain.ProviderOIDC
	}

	if err := user.Validate(); err != nil {
		return nil, fmt.Errorf("validate user: %w", err)
	}

	if err := s.store.Upsert(ctx, user); err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	if initialPassword != "" {
		hash, err := auth.HashPassword(initialPassword)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}

		if err := s.store.SetPassword(ctx, email, hash, true); err != nil {
			return nil, fmt.Errorf("set password: %w", err)
		}
	}

	return user, nil
}

func (s *Service) Delete(ctx context.Context, targetEmail string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(
		claims.Email,
		auth.ObjectAll,
		auth.ObjectUser,
		auth.ActionWrite,
	)
	if err != nil {
		return fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return domain.ErrForbidden
	}

	if claims.Email == targetEmail {
		return domain.NewValidationError("email", "cannot delete your own account")
	}

	if _, err := s.store.Get(ctx, targetEmail); err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	if err := s.validateLastAdmin(targetEmail); err != nil {
		return err
	}

	if err := s.store.Delete(ctx, targetEmail); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if err := s.enforcer.DeleteUser(targetEmail); err != nil {
		return fmt.Errorf("delete casbin user: %w", err)
	}

	return nil
}

func (s *Service) List(ctx context.Context) ([]*domain.User, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, auth.ObjectAll, auth.ObjectUser, auth.ActionRead)
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	users, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return users, nil
}

func (s *Service) Get(ctx context.Context, email string) (*domain.User, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	allowed, err := s.enforcer.Enforce(claims.Email, auth.ObjectAll, auth.ObjectUser, auth.ActionRead)
	if err != nil {
		return nil, fmt.Errorf("enforce: %w", err)
	}

	if !allowed {
		return nil, domain.ErrForbidden
	}

	user, err := s.store.Get(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}

func (s *Service) validateLastAdmin(targetEmail string) error {
	rules := s.enforcer.GetGroupingPolicy()
	adminCount := 0
	isTargetAdmin := false

	for _, rule := range rules {
		if len(rule) == 3 && rule[1] == auth.RoleAdmin && rule[2] == auth.ObjectAll {
			adminCount++
			if rule[0] == targetEmail {
				isTargetAdmin = true
			}
		}
	}

	if isTargetAdmin && adminCount == 1 {
		return domain.NewValidationError("email", "cannot delete the last admin")
	}

	return nil
}
