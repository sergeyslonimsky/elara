package user

import (
	"context"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	"github.com/sergeyslonimsky/elara/internal/service/storage"
)

// Authorization for all UserService methods is enforced in the handler layer
// (EL-4 M9: `(User, Create|Read|Write, *)`). The usecase still needs claims
// from context for `Delete` self-protection ("cannot delete your own account").

func (s *Service) Create(ctx context.Context, email, name, initialPassword string) (*domain.User, error) {
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

	if claims.Email == targetEmail {
		return domain.NewValidationError("email", "cannot delete your own account")
	}

	if _, err := s.store.Get(ctx, targetEmail); err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	if err := s.validateLastAdmin(targetEmail); err != nil {
		return err
	}

	err := s.pap.Write(ctx, func(tx storage.Tx, w *authz.PAPTx) error {
		if err := s.users.WithTx(tx).Delete(ctx, targetEmail); err != nil {
			return fmt.Errorf("delete user: %w", err)
		}

		if err := w.DeleteUser(targetEmail); err != nil {
			return fmt.Errorf("pap delete user: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("delete user transaction: %w", err)
	}

	return nil
}

// GetResult bundles a user with the canonical set of group IDs they
// currently belong to. Group IDs are returned irrespective of the caller's
// per-group read permission so the user edit UI can submit a full set back
// through UpdateUserGroups, which itself diffs and authorizes only the
// symmetric difference.
type GetResult struct {
	User     *domain.User
	GroupIDs []string
}

func (s *Service) Get(ctx context.Context, email string) (*GetResult, error) {
	user, err := s.store.Get(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	names, err := s.pap.UserGroupNames(email)
	if err != nil {
		return nil, fmt.Errorf("get user group names: %w", err)
	}

	ids := make([]string, 0, len(names))
	for _, name := range names {
		g, err := s.groups.FindByName(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("find group by name %s: %w", name, err)
		}
		ids = append(ids, g.ID)
	}

	return &GetResult{User: user, GroupIDs: ids}, nil
}

func (s *Service) validateLastAdmin(targetEmail string) error {
	if s.pap.HasDirectAdminAssignment(targetEmail) && s.pap.AdminAssignmentCount() == 1 {
		return domain.NewValidationError("email", "cannot delete the last admin")
	}

	return nil
}
