package domain

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type ValidationResult struct {
	Valid             bool
	Errors            []string
	DetectedFormat    Format
	NormalizedContent string
	SchemaViolations  []SchemaViolation
}

func ValidateContent(content string, format Format) error {
	switch format {
	case FormatJSON:
		return validateJSON(content)
	case FormatYAML:
		return validateYAML(content)
	case FormatOther:
		return nil
	default:
		return NewInvalidFormatError(string(format))
	}
}

func DetectFormat(content string) (Format, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", NewValidationError("content", "empty content")
	}

	if err := validateJSON(content); err == nil {
		return FormatJSON, nil
	}

	if err := validateYAML(content); err == nil {
		return FormatYAML, nil
	}

	return "", NewValidationError("content", "content is neither valid JSON nor YAML")
}

func NormalizeContent(content string, format Format) (string, error) {
	switch format {
	case FormatJSON:
		return normalizeJSON(content)
	case FormatYAML:
		return normalizeYAML(content)
	case FormatOther:
		return content, nil
	default:
		return content, nil
	}
}

func ValidateAndNormalize(content string, format Format) (*ValidationResult, error) {
	result := &ValidationResult{}

	actualFormat := format
	if actualFormat == "" {
		detected, err := DetectFormat(content)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())

			return result, nil
		}

		actualFormat = detected
	}

	result.DetectedFormat = actualFormat

	if err := ValidateContent(content, actualFormat); err != nil {
		result.Errors = append(result.Errors, err.Error())

		return result, nil
	}

	normalized, err := NormalizeContent(content, actualFormat)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())

		return result, nil
	}

	result.Valid = true
	result.NormalizedContent = normalized

	return result, nil
}

func validateJSON(content string) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(content), &js); err != nil {
		return fmt.Errorf("unmarshal JSON: %w", err)
	}

	return nil
}

func validateYAML(content string) error {
	var ys any
	if err := yaml.Unmarshal([]byte(content), &ys); err != nil {
		return fmt.Errorf("unmarshal YAML: %w", err)
	}

	return nil
}

func normalizeJSON(content string) (string, error) {
	var js any
	if err := json.Unmarshal([]byte(content), &js); err != nil {
		return "", fmt.Errorf("%w: unmarshal JSON: %w", ErrInvalidContent, err)
	}

	normalized, err := json.MarshalIndent(js, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	}

	return string(normalized), nil
}

func normalizeYAML(content string) (string, error) {
	var ys any
	if err := yaml.Unmarshal([]byte(content), &ys); err != nil {
		return "", fmt.Errorf("%w: unmarshal YAML: %w", ErrInvalidContent, err)
	}

	normalized, err := yaml.Marshal(ys)
	if err != nil {
		return "", fmt.Errorf("marshal YAML: %w", err)
	}

	return string(normalized), nil
}
