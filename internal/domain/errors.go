package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrAlreadyExists  = errors.New("already exists")
	ErrConflict       = errors.New("version conflict")
	ErrInvalidFormat  = errors.New("invalid format")
	ErrInvalidContent = errors.New("invalid content")
	ErrLocked         = errors.New("config is locked")
	// ErrNamespaceLocked wraps ErrLocked so callers can attribute the cause
	// (e.g. for metrics) while still matching errors.Is(err, ErrLocked).
	ErrNamespaceLocked = fmt.Errorf("namespace is locked: %w", ErrLocked)
)

type ValidationError struct {
	Field   string
	Message string
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation: %s: %s", e.Field, e.Message)
	}

	return "validation: " + e.Message
}

func IsValidationError(err error) bool {
	var ve *ValidationError

	return errors.As(err, &ve)
}

func NewLockedError(path string) error {
	return fmt.Errorf("config %q: %w", path, ErrLocked)
}

func NewInvalidFormatError(format string) error {
	return fmt.Errorf("%w: %s (supported: json, yaml)", ErrInvalidFormat, format)
}

func NewNotFoundError(resource, identifier string) error {
	return fmt.Errorf("%s %q: %w", resource, identifier, ErrNotFound)
}

func NewAlreadyExistsError(resource, identifier string) error {
	return fmt.Errorf("%s %q: %w", resource, identifier, ErrAlreadyExists)
}

func NewConflictError(expected, actual int64) error {
	return fmt.Errorf("expected version %d, got %d: %w", expected, actual, ErrConflict)
}
