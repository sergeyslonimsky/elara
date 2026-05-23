// Package content provides format detection, validation, and normalization
// for raw configuration content (JSON / YAML / opaque). It lives outside
// the domain package because it depends on yaml/json parsers — domain stays
// free of infrastructure imports.
package content

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// ValidationResult is the structured outcome of ValidateAndNormalize: the
// detected format, normalized content, and either a list of human-readable
// errors or the JSON-schema violations recorded by upstream callers.
type ValidationResult struct {
	Valid             bool
	Errors            []string
	DetectedFormat    domain.Format
	NormalizedContent string
	SchemaViolations  []domain.SchemaViolation
}

// Validate parses content according to the given format and returns nil on
// success or an error describing the parse failure.
func Validate(content string, format domain.Format) error {
	switch format {
	case domain.FormatJSON:
		return validateJSON(content)
	case domain.FormatYAML:
		return validateYAML(content)
	case domain.FormatOther:
		return nil
	default:
		return fmt.Errorf("validate content: %w", domain.NewInvalidFormatError(string(format)))
	}
}

// DetectFormat probes the content as JSON first, then YAML, and returns the
// first format that parses cleanly.
func DetectFormat(content string) (domain.Format, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", domain.NewValidationError("content", "empty content")
	}

	if err := validateJSON(content); err == nil {
		return domain.FormatJSON, nil
	}

	if err := validateYAML(content); err == nil {
		return domain.FormatYAML, nil
	}

	return "", domain.NewValidationError("content", "content is neither valid JSON nor YAML")
}

// Normalize re-serializes the content in its canonical form for the given
// format (indented JSON / canonical YAML).
func Normalize(content string, format domain.Format) (string, error) {
	switch format {
	case domain.FormatJSON:
		return normalizeJSON(content)
	case domain.FormatYAML:
		return normalizeYAML(content)
	case domain.FormatOther:
		return content, nil
	default:
		return content, nil
	}
}

// ValidateAndNormalize is the one-shot entry point: detects the format (if
// not specified), validates, normalizes, and returns a ValidationResult.
// On parse errors it returns the result with Valid=false rather than an
// error — invalid content is an expected outcome for this API.
func ValidateAndNormalize(content string, format domain.Format) (*ValidationResult, error) {
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

	if err := Validate(content, actualFormat); err != nil {
		result.Errors = append(result.Errors, err.Error())

		return result, nil
	}

	normalized, err := Normalize(content, actualFormat)
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
		return "", fmt.Errorf("%w: unmarshal JSON: %w", domain.ErrInvalidContent, err)
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
		return "", fmt.Errorf("%w: unmarshal YAML: %w", domain.ErrInvalidContent, err)
	}

	normalized, err := yaml.Marshal(ys)
	if err != nil {
		return "", fmt.Errorf("marshal YAML: %w", err)
	}

	return string(normalized), nil
}
