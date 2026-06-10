package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

func validSessionEvent() domain.SessionEvent {
	return domain.SessionEvent{
		ID:        "evt-1",
		SessionID: "sess-1",
		UserID:    "user-1",
		Type:      domain.SessionEventCreated,
		Timestamp: time.Now(),
	}
}

func TestSessionEvent_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		event   domain.SessionEvent
		wantErr bool
		errMsg  string
	}{
		{
			name:  "valid event",
			event: validSessionEvent(),
		},
		{
			name: "empty Type",
			event: func() domain.SessionEvent {
				e := validSessionEvent()
				e.Type = ""

				return e
			}(),
			wantErr: true,
			errMsg:  "type",
		},
		{
			name: "empty SessionID",
			event: func() domain.SessionEvent {
				e := validSessionEvent()
				e.SessionID = ""

				return e
			}(),
			wantErr: true,
			errMsg:  "sessionId",
		},
		{
			name: "empty UserID",
			event: func() domain.SessionEvent {
				e := validSessionEvent()
				e.UserID = ""

				return e
			}(),
			wantErr: true,
			errMsg:  "userId",
		},
		{
			name: "zero Timestamp",
			event: func() domain.SessionEvent {
				e := validSessionEvent()
				e.Timestamp = time.Time{}

				return e
			}(),
			wantErr: true,
			errMsg:  "timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.event.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.True(t, domain.IsValidationError(err))

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNewEventID(t *testing.T) {
	t.Parallel()

	t.Run("produces non-empty output", func(t *testing.T) {
		t.Parallel()

		id, err := domain.NewEventID()
		require.NoError(t, err)
		assert.NotEmpty(t, id)
	})

	t.Run("uniqueness across 200 calls", func(t *testing.T) {
		t.Parallel()

		seen := make(map[string]struct{}, 200)
		for range 200 {
			id, err := domain.NewEventID()
			require.NoError(t, err)
			seen[id] = struct{}{}
		}
		assert.Len(t, seen, 200)
	})
}
