package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound             = errors.New("not found")
	ErrAlreadyExists        = errors.New("already exists")
	ErrVersionConflict      = errors.New("version conflict")
	ErrSystemImmutable      = errors.New("system entity is immutable")
	ErrPermissionEscalation = errors.New("permission escalation")
	ErrInvalidFormat        = errors.New("invalid format")
	ErrInvalidContent       = errors.New("invalid content")
	ErrLocked               = errors.New("config is locked")
	// ErrNamespaceLocked wraps ErrLocked so callers can attribute the cause
	// (e.g. for metrics) while still matching errors.Is(err, ErrLocked).
	ErrNamespaceLocked        = fmt.Errorf("namespace is locked: %w", ErrLocked)
	ErrUnauthorized           = errors.New("unauthorized")
	ErrForbidden              = errors.New("forbidden")
	ErrInvalidToken           = errors.New("invalid token")
	ErrPasswordChangeRequired = errors.New("password change required")
	ErrFeatureNotAvailable    = errors.New("feature not available")
	ErrSessionNotFound        = errors.New("session not found")
	ErrSessionExpired         = errors.New("session expired")
	ErrSessionRevoked         = errors.New("session revoked")
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
	return fmt.Errorf("expected version %d, got %d: %w", expected, actual, ErrVersionConflict)
}

// CheckVersion compares the optional caller-supplied expected version
// against the current value. Nil expected means the caller opted out of
// optimistic concurrency; matching values pass; a mismatch returns
// ErrVersionConflict.
//
// Lives here rather than in a usecase package so every Update* flow that
// exposes optimistic locking shares the same contract.
func CheckVersion(expected *int64, current int64) error {
	if expected == nil {
		return nil
	}
	if *expected != current {
		return ErrVersionConflict
	}

	return nil
}
