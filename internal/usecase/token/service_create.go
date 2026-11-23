package token

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type CreateInput struct {
	Name       string // prefix of the generated Username
	Namespaces []string
	Role       domain.Role
	ExpiresAt  *time.Time
}

func (s *Service) Create(
	ctx context.Context,
	user domain.AuthInfo,
	in CreateInput,
) (*domain.Token, string, error) {
	if len(in.Namespaces) == 0 {
		return nil, "", domain.NewValidationError(
			"namespaces",
			"at least one namespace is required",
		)
	}

	// Role-scope invariant: a token cannot grant more than its creator has on
	// each namespace. A reader-role token requires (Namespace, Read, ns); a
	// writer requires (Namespace, Write, ns). Handler already verified
	// (Token, Create, ns); this is the analogue of UpdateGroup's diff-boundary check.
	action, err := configActionForRole(in.Role)
	if err != nil {
		return nil, "", err
	}
	for _, ns := range in.Namespaces {
		if !s.pdp.HasNamespace(user.UserID, ns, action) {
			return nil, "", domain.ErrPermissionEscalation
		}
	}

	rawToken, tokenHash, err := generateRawToken()
	if err != nil {
		return nil, "", err
	}

	if err := domain.ValidateTokenNamePrefix(in.Name); err != nil {
		return nil, "", fmt.Errorf("validate: %w", err)
	}

	token := &domain.Token{
		ID:         uuid.New().String(),
		IssuedBy:   user.Email,
		Name:       in.Name,
		TokenHash:  tokenHash,
		Namespaces: in.Namespaces,
		Role:       in.Role,
		ExpiresAt:  in.ExpiresAt,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.createWithUsername(ctx, token); err != nil {
		return nil, "", err
	}

	return token, rawToken, nil
}

// createWithUsername stores token under a freshly generated Username. The
// Username is unique by construction; a suffix collision is resolved by
// drawing a new one, never by reporting it to the caller — that would tell
// them a token they may not see exists.
func (s *Service) createWithUsername(ctx context.Context, token *domain.Token) error {
	for range maxUsernameAttempts {
		token.Username = domain.NewTokenUsername(token.Name, generateUsernameSuffix())

		if err := token.Validate(); err != nil {
			return fmt.Errorf("validate: %w", err)
		}

		err := s.store.Create(ctx, token)
		if errors.Is(err, domain.ErrTokenUsernameTaken) {
			continue
		}
		if err != nil {
			return fmt.Errorf("create token: %w", err)
		}

		return nil
	}

	return fmt.Errorf("create token: %w", domain.ErrTokenUsernameTaken)
}

// configActionForRole maps a token role to the (Config, action) the creator
// must hold on each requested namespace.
func configActionForRole(role domain.Role) (domain.Action, error) {
	switch role {
	case domain.RoleReader:
		return domain.ActionRead, nil
	case domain.RoleWriter:
		return domain.ActionWrite, nil
	default:
		return "", domain.NewValidationError("role", "must be reader or writer")
	}
}
