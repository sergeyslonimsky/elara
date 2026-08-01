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

func TestObjectGrants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		granted  domain.Object
		required domain.Object
		want     bool
	}{
		{
			name: "wildcard grants any object", granted: domain.ObjectAll,
			required: domain.ObjectNamespace, want: true,
		},
		{
			name: "exact match grants", granted: domain.ObjectToken,
			required: domain.ObjectToken, want: true,
		},
		{
			name: "mismatched objects do not grant", granted: domain.ObjectToken,
			required: domain.ObjectNamespace, want: false,
		},
		{
			name:    "wildcard required does not itself grant a concrete object",
			granted: domain.ObjectToken, required: domain.ObjectAll, want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.ObjectGrants(tt.granted, tt.required)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestActionGrants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		granted  domain.Action
		required domain.Action
		want     bool
	}{
		{name: "wildcard grants any action", granted: domain.ActionAll, required: domain.ActionDelete, want: true},
		{name: "exact match grants", granted: domain.ActionRead, required: domain.ActionRead, want: true},
		{name: "write implies read", granted: domain.ActionWrite, required: domain.ActionRead, want: true},
		{name: "read does not imply write", granted: domain.ActionRead, required: domain.ActionWrite, want: false},
		{
			name:     "unrelated actions do not grant",
			granted:  domain.ActionCreate,
			required: domain.ActionDelete,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.ActionGrants(tt.granted, tt.required)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGroupResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "concrete name gets prefixed", in: "editors", want: "group:editors"},
		{name: "wildcard passes through unchanged", in: domain.DomainAll, want: domain.DomainAll},
		{name: "empty name still gets prefixed", in: "", want: "group:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.GroupResource(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNamespaceResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "concrete name gets prefixed", in: "prod", want: "namespace:prod"},
		{name: "wildcard passes through unchanged", in: domain.DomainAll, want: domain.DomainAll},
		{name: "empty name still gets prefixed", in: "", want: "namespace:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.NamespaceResource(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsGroupSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject string
		want    bool
	}{
		{name: "group subject", subject: "group:editors", want: true},
		{name: "bare user UUID is not a group", subject: "3f8e1c2a-1111-2222-3333-444455556666", want: false},
		{name: "empty subject is not a group", subject: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.IsGroupSubject(tt.subject)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGroupNameFromSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject string
		want    string
	}{
		{name: "strips group prefix", subject: "group:editors", want: "editors"},
		{name: "non-group subject returned unchanged", subject: "some-user-id", want: "some-user-id"},
		{name: "empty subject returns empty", subject: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.GroupNameFromSubject(tt.subject)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsUserSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject string
		want    bool
	}{
		{name: "bare user UUID is a user subject", subject: "3f8e1c2a-1111-2222-3333-444455556666", want: true},
		{name: "group subject is not a user subject", subject: "group:editors", want: false},
		{name: "empty subject is a user subject", subject: "", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.IsUserSubject(tt.subject)
			assert.Equal(t, tt.want, got)
		})
	}
}
