package domain

import (
	"fmt"
	"regexp"
)

var canonicalNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

const (
	maxCanonicalNameLen = 63 // DNS-1123 label limit (RFC 1123 §2.1)
	maxDisplayNameLen   = 128
)

func ValidateCanonicalName(field, value string) error {
	if value == "" {
		return NewValidationError(field, field+" is required")
	}

	if len(value) > maxCanonicalNameLen {
		return NewValidationError(
			field,
			fmt.Sprintf("%s must be at most %d characters", field, maxCanonicalNameLen),
		)
	}

	if !canonicalNameRegex.MatchString(value) {
		return NewValidationError(
			field,
			field+" must be a valid DNS-1123 label (lowercase alphanumeric and hyphens, starting/ending with alphanumeric)",
		)
	}

	return nil
}
