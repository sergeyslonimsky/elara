package internal_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storageinternal "github.com/sergeyslonimsky/elara/internal/storage/internal"
)

func TestDomainToSessionMeta(t *testing.T) {
	t.Parallel()

	now := time.Now()
	revokedAt := now.Add(time.Hour)

	tests := []struct {
		name string
		sess *domain.Session
		want storageinternal.SessionMeta
	}{
		{
			name: "active session",
			sess: &domain.Session{
				ID:         "sess-1",
				UserID:     "user-1",
				ClientType: domain.ClientTypeWeb,
				IP:         "1.2.3.4",
				UserAgent:  "ua",
				CreatedAt:  now,
				LastSeenAt: now,
				ExpiresAt:  now.Add(8 * time.Hour),
			},
			want: storageinternal.SessionMeta{
				ID:         "sess-1",
				UserID:     "user-1",
				ClientType: "web",
				IP:         "1.2.3.4",
				UserAgent:  "ua",
				CreatedAt:  now,
				LastSeenAt: now,
				ExpiresAt:  now.Add(8 * time.Hour),
			},
		},
		{
			name: "revoked session",
			sess: &domain.Session{
				ID:         "sess-2",
				UserID:     "user-2",
				ClientType: domain.ClientTypeCLI,
				RevokedAt:  &revokedAt,
				RevokedBy:  "admin",
			},
			want: storageinternal.SessionMeta{
				ID:         "sess-2",
				UserID:     "user-2",
				ClientType: "cli",
				RevokedAt:  &revokedAt,
				RevokedBy:  "admin",
			},
		},
		{
			name: "zero value session",
			sess: &domain.Session{},
			want: storageinternal.SessionMeta{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.DomainToSessionMeta(tt.sess)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSessionMetaToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now()
	revokedAt := now.Add(time.Hour)

	tests := []struct {
		name string
		meta storageinternal.SessionMeta
		want *domain.Session
	}{
		{
			name: "active session",
			meta: storageinternal.SessionMeta{
				ID:         "sess-1",
				UserID:     "user-1",
				ClientType: "web",
				IP:         "1.2.3.4",
				UserAgent:  "ua",
				CreatedAt:  now,
				LastSeenAt: now,
				ExpiresAt:  now.Add(8 * time.Hour),
			},
			want: &domain.Session{
				ID:         "sess-1",
				UserID:     "user-1",
				ClientType: domain.ClientTypeWeb,
				IP:         "1.2.3.4",
				UserAgent:  "ua",
				CreatedAt:  now,
				LastSeenAt: now,
				ExpiresAt:  now.Add(8 * time.Hour),
			},
		},
		{
			name: "revoked session",
			meta: storageinternal.SessionMeta{
				ID:         "sess-2",
				ClientType: "cli",
				RevokedAt:  &revokedAt,
				RevokedBy:  "admin",
			},
			want: &domain.Session{
				ID:         "sess-2",
				ClientType: domain.ClientTypeCLI,
				RevokedAt:  &revokedAt,
				RevokedBy:  "admin",
			},
		},
		{
			name: "zero value meta",
			meta: storageinternal.SessionMeta{},
			want: &domain.Session{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.SessionMetaToDomain(tt.meta)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDomainToSessionEventMeta(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name  string
		event *domain.SessionEvent
		want  storageinternal.SessionEventMeta
	}{
		{
			name: "full event",
			event: &domain.SessionEvent{
				ID:        "evt-1",
				SessionID: "sess-1",
				UserID:    "user-1",
				Type:      domain.SessionEventCreated,
				Reason:    "login",
				IP:        "1.2.3.4",
				UserAgent: "ua",
				Timestamp: now,
			},
			want: storageinternal.SessionEventMeta{
				ID:        "evt-1",
				SessionID: "sess-1",
				UserID:    "user-1",
				Type:      "created",
				Reason:    "login",
				IP:        "1.2.3.4",
				UserAgent: "ua",
				Timestamp: now,
			},
		},
		{
			name:  "zero value event",
			event: &domain.SessionEvent{},
			want:  storageinternal.SessionEventMeta{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.DomainToSessionEventMeta(tt.event)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSessionEventMetaToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name string
		meta storageinternal.SessionEventMeta
		want *domain.SessionEvent
	}{
		{
			name: "full event",
			meta: storageinternal.SessionEventMeta{
				ID:        "evt-1",
				SessionID: "sess-1",
				UserID:    "user-1",
				Type:      "revoked_by_admin",
				Reason:    "policy",
				IP:        "1.2.3.4",
				UserAgent: "ua",
				Timestamp: now,
			},
			want: &domain.SessionEvent{
				ID:        "evt-1",
				SessionID: "sess-1",
				UserID:    "user-1",
				Type:      domain.SessionEventRevokedByAdmin,
				Reason:    "policy",
				IP:        "1.2.3.4",
				UserAgent: "ua",
				Timestamp: now,
			},
		},
		{
			name: "zero value meta",
			meta: storageinternal.SessionEventMeta{},
			want: &domain.SessionEvent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.SessionEventMetaToDomain(tt.meta)
			assert.Equal(t, tt.want, got)
		})
	}
}
