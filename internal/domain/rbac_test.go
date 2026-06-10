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
	assert.Equal(t, domain.ActionCreate, domain.Action("create"))
	assert.Equal(t, domain.ActionRead, domain.Action("read"))
	assert.Equal(t, domain.ActionWrite, domain.Action("write"))
	assert.Equal(t, domain.ActionAll, domain.Action("*"))
}

func TestActionConstants_NoCollisions(t *testing.T) {
	t.Parallel()

	// Each action constant must be distinct — collisions would make
	// authorization checks ambiguous.
	actions := map[string]domain.Action{
		"ActionAll":    domain.ActionAll,
		"ActionCreate": domain.ActionCreate,
		"ActionRead":   domain.ActionRead,
		"ActionWrite":  domain.ActionWrite,
	}

	seen := make(map[domain.Action]string, len(actions))
	for name, value := range actions {
		if existing, ok := seen[value]; ok {
			t.Fatalf("action constant collision: %s and %s both equal %q", existing, name, value)
		}
		seen[value] = name
	}

	assert.Len(t, seen, len(actions))
}
