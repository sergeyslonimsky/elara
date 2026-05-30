package domain

import (
	"fmt"
	"time"
)

const (
	maxGroupNameLen   = 128
	maxDescriptionLen = 1024
)

// GroupFilter narrows a group list to the names visible to the caller.
//
// When Wildcard is true, Names MUST be ignored — every group matches.
// Search applies a case-insensitive substring match on Name and is
// independent of Wildcard/Names.
type GroupFilter struct {
	Names    map[string]struct{}
	Wildcard bool
	Search   string
}

// GroupListParams carries pagination and sort options for group list queries.
type GroupListParams struct {
	Limit  int
	Offset int
	Sort   SortParams
}

// Group is the bbolt-persisted entity. Members and permissions live in
// Casbin (g-rules / p-rules); the service layer composes them with this
// entity at the response boundary and they never round-trip through bbolt.
//
// Three independent optimistic-lock counters let concurrent edits to
// metadata, members, and permissions proceed without false conflicts —
// see the proto comment on the wire-level message for the full contract.
type Group struct {
	ID                 string
	Name               string
	Description        string
	System             bool // protected from delete/rename; set by Seed, never by the API
	MetadataVersion    int64
	MembersVersion     int64
	PermissionsVersion int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (g *Group) EnsureMutable() error {
	if g.System {
		return ErrSystemImmutable
	}

	return nil
}

func (g *Group) Validate() error {
	if g.ID == "" {
		return NewValidationError("id", "group id is required")
	}

	if g.Name == "" {
		return NewValidationError("name", "group name is required")
	}

	if len(g.Name) > maxGroupNameLen {
		return NewValidationError(
			"name",
			fmt.Sprintf("group name must be at most %d characters", maxGroupNameLen),
		)
	}

	if len(g.Description) > maxDescriptionLen {
		return NewValidationError(
			"description",
			fmt.Sprintf("group description must be at most %d characters", maxDescriptionLen),
		)
	}

	return nil
}
