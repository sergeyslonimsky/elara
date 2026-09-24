package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func TestEventTypeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event domain.EventType
		want  string
	}{
		{
			name:  "created",
			event: domain.EventTypeCreated,
			want:  "CREATED",
		},
		{
			name:  "updated",
			event: domain.EventTypeUpdated,
			want:  "UPDATED",
		},
		{
			name:  "deleted",
			event: domain.EventTypeDeleted,
			want:  "DELETED",
		},
		{
			name:  "locked",
			event: domain.EventTypeLocked,
			want:  "LOCKED",
		},
		{
			name:  "unlocked",
			event: domain.EventTypeUnlocked,
			want:  "UNLOCKED",
		},
		{
			name:  "namespace locked",
			event: domain.EventTypeNamespaceLocked,
			want:  "NAMESPACE_LOCKED",
		},
		{
			name:  "namespace unlocked",
			event: domain.EventTypeNamespaceUnlocked,
			want:  "NAMESPACE_UNLOCKED",
		},
		{
			// The constants start at iota+1, so the zero value is not a valid
			// event — an EventType left unset must not render as CREATED.
			name:  "zero value",
			event: domain.EventType(0),
			want:  "UNKNOWN",
		},
		{
			name:  "above the last constant",
			event: domain.EventType(8),
			want:  "UNKNOWN",
		},
		{
			name:  "negative",
			event: domain.EventType(-1),
			want:  "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.event.String())
		})
	}
}

// TestEventTypeDistinct asserts a property rather than a mapping: every event
// type renders as its own non-default string.
//
// This is what TestEventTypeString cannot do. That table only knows about the
// constants someone remembered to list in it, so a new EventType added to the
// const block and wired into the system — but never given a case in String() —
// leaves it green while the event renders as "UNKNOWN" on the wire.
//
// It also pins the hazard called out on the const block in watch.go: the
// single-config and namespace-scope lock events are interchangeable at compile
// time, so a copy-paste that leaves two of them returning the same string
// "fails silently" everywhere else.
//
// The residual gap is unavoidable without reflection: a constant that nobody
// adds to all below is still invisible here.
func TestEventTypeDistinct(t *testing.T) {
	t.Parallel()

	all := []domain.EventType{
		domain.EventTypeCreated,
		domain.EventTypeUpdated,
		domain.EventTypeDeleted,
		domain.EventTypeLocked,
		domain.EventTypeUnlocked,
		domain.EventTypeNamespaceLocked,
		domain.EventTypeNamespaceUnlocked,
	}

	seen := make(map[string]domain.EventType, len(all))

	for _, event := range all {
		got := event.String()

		assert.NotEqualf(t, "UNKNOWN", got,
			"EventType(%d) falls through to the default arm of String()", int(event))

		if previous, duplicate := seen[got]; duplicate {
			assert.Failf(t, "event types share a string representation",
				"EventType(%d) and EventType(%d) both render as %q",
				int(previous), int(event), got)

			continue
		}

		seen[got] = event
	}

	assert.Len(t, seen, len(all))
}
