package domain

import (
	"errors"
	"fmt"
	"strings"
)

type SchemaViolation struct {
	Path    string
	Message string
	Keyword string
}

type SchemaValidationError struct {
	Violations []SchemaViolation
}

func (e *SchemaValidationError) Error() string {
	parts := make([]string, 0, len(e.Violations))
	for _, v := range e.Violations {
		parts = append(parts, fmt.Sprintf("%s: %s [%s]", v.Path, v.Message, v.Keyword))
	}

	return fmt.Sprintf(
		"schema validation failed: %d violation(s): %s",
		len(e.Violations),
		strings.Join(parts, "; "),
	)
}

func IsSchemaValidationError(err error) bool {
	var sve *SchemaValidationError

	return errors.As(err, &sve)
}
