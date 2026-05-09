package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
)

type ValidateInput struct {
	Content   string
	Format    domain.Format
	Namespace string
	Path      string
}

func (s *Service) Validate(ctx context.Context, in ValidateInput) (*domain.ValidationResult, error) {
	// Only enforce when namespace is provided (schema validation path).
	if in.Namespace != "" {
		if err := auth.CheckAccess(ctx, s.enforcer, in.Namespace, auth.ObjectConfig, auth.ActionRead); err != nil {
			return nil, fmt.Errorf("check access: %w", err)
		}
	} else if _, ok := auth.ClaimsFromContext(ctx); !ok {
		return nil, domain.ErrUnauthorized
	}

	result, err := domain.ValidateAndNormalize(in.Content, in.Format)
	if err != nil {
		return nil, fmt.Errorf("validate and normalize: %w", err)
	}

	if !result.Valid || in.Namespace == "" || in.Path == "" {
		return result, nil
	}

	if err := s.schemaValidator.Validate(
		ctx,
		in.Namespace,
		in.Path,
		result.NormalizedContent,
		result.DetectedFormat,
	); err != nil {
		if sve, ok := errors.AsType[*domain.SchemaValidationError](err); ok {
			result.Valid = false
			result.SchemaViolations = sve.Violations

			return result, nil
		}

		return nil, fmt.Errorf("schema validation: %w", err)
	}

	return result, nil
}
