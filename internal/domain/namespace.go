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
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
