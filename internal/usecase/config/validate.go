package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

type validateEnforcer interface {
	Enforce(subject, domain, object, action string) (bool, error)
}

type validateSchemaChecker interface {
	Execute(ctx context.Context, namespace, configPath, content string, format domain.Format) error
}

type ValidateUseCase struct {
	enforcer validateEnforcer
	schema   validateSchemaChecker
}

func NewValidateUseCase(enforcer validateEnforcer, schema validateSchemaChecker) *ValidateUseCase {
	return &ValidateUseCase{enforcer: enforcer, schema: schema}
}

func (uc *ValidateUseCase) Execute(
	ctx context.Context,
	content string,
	format domain.Format,
	namespace, path string,
) (*domain.ValidationResult, error) {
	// Only enforce when namespace is provided (schema validation path).
	if namespace != "" {
		if err := auth.CheckAccess(ctx, uc.enforcer, namespace, auth.ObjectConfig, auth.ActionRead); err != nil {
			return nil, fmt.Errorf("check access: %w", err)
		}
	} else if _, ok := auth.ClaimsFromContext(ctx); !ok {
		return nil, domain.ErrUnauthorized
	}

	result, err := domain.ValidateAndNormalize(content, format)
	if err != nil {
		return nil, fmt.Errorf("validate and normalize: %w", err)
	}

	if !result.Valid || namespace == "" || path == "" {
		return result, nil
	}

	if err := uc.schema.Execute(ctx, namespace, path, result.NormalizedContent, result.DetectedFormat); err != nil {
		if sve, ok := errors.AsType[*domain.SchemaValidationError](err); ok {
			result.Valid = false
			result.SchemaViolations = sve.Violations

			return result, nil
		}

		return nil, fmt.Errorf("schema validation: %w", err)
	}

	return result, nil
}
