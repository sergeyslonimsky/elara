package domain

import (
	"fmt"
	"time"
)

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
	DisplayName string
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
	if err := ValidateCanonicalName("name", n.Name); err != nil {
		return err
	}

	if len(n.DisplayName) > maxDisplayNameLen {
		return NewValidationError(
			"displayName",
			fmt.Sprintf("display name must be at most %d characters", maxDisplayNameLen),
		)
	}

	return nil
}
