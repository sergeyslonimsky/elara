package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/content"
)

type ValidateInput struct {
	Content   string
	Format    domain.Format
	Namespace string
	Path      string
}

func (s *Service) Validate(ctx context.Context, in ValidateInput) (*content.ValidationResult, error) {
	result, err := content.ValidateAndNormalize(in.Content, in.Format)
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
