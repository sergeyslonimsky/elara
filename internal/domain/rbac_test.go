package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestActionConstants(t *testing.T) {
	t.Parallel()

	// Lock in the exact wire values — typos here would silently break
	// Casbin policy strings and proto enum mapping.
	assert.Equal(t, "create", domain.ActionCreate)
	assert.Equal(t, "read", domain.ActionRead)
	assert.Equal(t, "write", domain.ActionWrite)
	assert.Equal(t, "*", domain.ActionAll)
}

func TestActionConstants_NoCollisions(t *testing.T) {
	t.Parallel()

	// Each action constant must be distinct — collisions would make
	// authorization checks ambiguous.
	actions := map[string]string{
		"ActionAll":    domain.ActionAll,
		"ActionCreate": domain.ActionCreate,
		"ActionRead":   domain.ActionRead,
		"ActionWrite":  domain.ActionWrite,
	}

	seen := make(map[string]string, len(actions))
	for name, value := range actions {
		if existing, ok := seen[value]; ok {
			t.Fatalf("action constant collision: %s and %s both equal %q", existing, name, value)
		}
		seen[value] = name
	}

	assert.Len(t, seen, len(actions))
}
