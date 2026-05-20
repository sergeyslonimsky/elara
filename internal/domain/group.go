package domain

import (
	"fmt"
	"slices"
	"strings"
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

type Group struct {
	ID          string
	Name        string
	Description string
	Members     []string // emails
	System      bool     // protected from delete/rename; set by Seed, never by the API
	Version     int64    // for optimistic locking
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

	for _, email := range g.Members {
		if email == "" || !strings.Contains(email, "@") {
			return NewValidationError("members", "member email must be a valid email address")
		}
	}

	return nil
}

func (g *Group) AddMember(email string) error {
	if email == "" || !strings.Contains(email, "@") {
		return NewValidationError("email", "email must be a valid email address")
	}

	if slices.Contains(g.Members, email) {
		return NewAlreadyExistsError("member", email)
	}

	g.Members = append(g.Members, email)

	return nil
}

func (g *Group) RemoveMember(email string) error {
	for i, m := range g.Members {
		if m == email {
			g.Members = append(g.Members[:i], g.Members[i+1:]...)

			return nil
		}
	}

	return NewNotFoundError("member", email)
}
