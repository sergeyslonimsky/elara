package config

import (
	"fmt"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type ValidateUseCase struct{}

func NewValidateUseCase() *ValidateUseCase {
	return &ValidateUseCase{}
}

func (uc *ValidateUseCase) Execute(content string, format domain.Format) (*domain.ValidationResult, error) {
	result, err := domain.ValidateAndNormalize(content, format)
	if err != nil {
		return nil, fmt.Errorf("validate and normalize: %w", err)
	}

	return result, nil
}
