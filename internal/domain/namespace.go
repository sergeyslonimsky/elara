package domain

import (
	"regexp"
	"time"
)

var namespaceNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`)

const maxNamespaceNameLen = 128

// NamespaceFilter narrows a namespace list to the subset visible to the caller.
//
// When Wildcard is true, Names MUST be ignored — the filter matches every
// namespace in storage. Search, when non-empty, applies a case-insensitive
// substring match against Namespace.Name and is independent of Wildcard/Names.
//
// Repo implementations MUST branch on Wildcard first to avoid unnecessary
// point-lookups when the caller has global read access.
type NamespaceFilter struct {
	Names    map[string]struct{}
	Wildcard bool
	Search   string
}

// NamespaceListParams carries pagination and sort options for namespace list
// queries. It is consumed both by the use case (which forwards it) and the
// storage adapter (which applies it after filter + search + sort).
type NamespaceListParams struct {
	Limit  int
	Offset int
	Sort   SortParams
}

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
