package domain

import (
	"regexp"
	"time"
)

var namespaceNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`)

const maxNamespaceNameLen = 128

type Namespace struct {
	Name        string
	Description string
	ConfigCount int
	Locked      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// CanRead / CanWrite are populated per request, not persisted. They tell
	// the UI what the *current caller* is allowed to do on this namespace —
	// e.g. CanWrite gates the "Import here" button in ListNamespaces.
	// Persistence adapters (bbolt) ignore these fields.
	CanRead  bool
	CanWrite bool
}

func (n *Namespace) Validate() error {
	if n.Name == "" {
		return NewValidationError("name", "namespace name is required")
	}

	if len(n.Name) > maxNamespaceNameLen {
		return NewValidationError("name", "namespace name must be at most 128 characters")
	}

	if !namespaceNameRegex.MatchString(n.Name) {
		return NewValidationError(
			"name",
			"namespace name must be alphanumeric with hyphens or underscores, starting with alphanumeric",
		)
	}

	return nil
}
