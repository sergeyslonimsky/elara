package domain

import (
	"strings"
	"time"
)

const maxGroupNameLen = 128

type Group struct {
	ID        string
	Name      string
	Members   []string // emails
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (g *Group) Validate() error {
	if g.ID == "" {
		return NewValidationError("id", "group id is required")
	}

	if g.Name == "" {
		return NewValidationError("name", "group name is required")
	}

	if len(g.Name) > maxGroupNameLen {
		return NewValidationError("name", "group name must be at most 128 characters")
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

	for _, m := range g.Members {
		if m == email {
			return NewAlreadyExistsError("member", email)
		}
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
