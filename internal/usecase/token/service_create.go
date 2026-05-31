package token

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type CreateInput struct {
	Name       string
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
		if !s.pdp.HasNamespace(user.Email, ns, action) {
			return nil, "", domain.ErrPermissionEscalation
		}
	}

	rawToken, tokenHash, err := generateRawToken()
	if err != nil {
		return nil, "", err
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

	if err := token.Validate(); err != nil {
		return nil, "", fmt.Errorf("validate: %w", err)
	}

	if err = s.store.Create(ctx, token); err != nil {
		return nil, "", fmt.Errorf("create token: %w", err)
	}

	return token, rawToken, nil
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
