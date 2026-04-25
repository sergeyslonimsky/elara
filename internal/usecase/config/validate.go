package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type validateSchemaChecker interface {
	Execute(ctx context.Context, namespace, configPath, content string, format domain.Format) error
}

type ValidateUseCase struct {
	schema validateSchemaChecker
}

func NewValidateUseCase(schema validateSchemaChecker) *ValidateUseCase {
	return &ValidateUseCase{schema: schema}
}

func (uc *ValidateUseCase) Execute(
	ctx context.Context,
	content string,
	format domain.Format,
	namespace, path string,
) (*domain.ValidationResult, error) {
	result, err := domain.ValidateAndNormalize(content, format)
	if err != nil {
		return nil, fmt.Errorf("validate and normalize: %w", err)
	}

	if !result.Valid || namespace == "" || path == "" {
		return result, nil
	}

	if err := uc.schema.Execute(ctx, namespace, path, result.NormalizedContent, result.DetectedFormat); err != nil {
		var sve *domain.SchemaValidationError
		if errors.As(err, &sve) {
			result.Valid = false
			result.SchemaViolations = sve.Violations

			return result, nil
		}

		return nil, fmt.Errorf("schema validation: %w", err)
	}

	return result, nil
}
