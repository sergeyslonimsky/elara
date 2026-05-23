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

// Group is the bbolt-persisted entity plus a transient view of its
// membership. Members is the source-of-truth view from Casbin g-rules
// (`g, <email>, group:<name>, "*"`) and is populated by the service layer
// after fetch — it is never written back to bbolt. Use the membership
// usecases (UpdateGroupMembers, UpdateUserGroups) to mutate it.
type Group struct {
	ID          string
	Name        string
	Description string
	Members     []string // emails — transient, enriched from Casbin
	System      bool     // protected from delete/rename; set by Seed, never by the API
	Version     int64    // for optimistic locking on metadata only
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SystemUserSuperAdmin is the username of the break-glass superuser.
const SystemUserSuperAdmin = "superadmin"

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
		return NewValidationError("name", fmt.Sprintf("group name must be at most %d characters", maxGroupNameLen))
	}

	if len(g.Description) > maxDescriptionLen {
		return NewValidationError(
			"description",
			fmt.Sprintf("group description must be at most %d characters", maxDescriptionLen),
		)
	}

	return nil
}
