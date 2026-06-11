package domain

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

type SessionEventType string

// SessionEvent log records ACTIVE state mutations only:
//   - Created: a session was minted (login).
//   - Refreshed: sliding-TTL extension applied (web sessions only).
//   - RevokedByUser / RevokedByAdmin / RevokedCascade: an explicit caller
//     terminated the session.
//
// Passive expiration is NOT logged: when ExpiresAt < now the session
// simply stops authenticating. The expiration moment is observable
// directly from sess.ExpiresAt in the row; the audit log captures
// observed actions, not time-driven transitions. An "expired" event
// type was scoped during EL-49 design but never implemented — the
// product call (2026-06) is that login events suffice for the
// forensics use case and idle-expired sessions don't need a paper trail.
const (
	SessionEventCreated        SessionEventType = "created"
	SessionEventRefreshed      SessionEventType = "refreshed"
	SessionEventRevokedByUser  SessionEventType = "revoked_by_user"
	SessionEventRevokedByAdmin SessionEventType = "revoked_by_admin"
	SessionEventRevokedCascade SessionEventType = "revoked_cascade"
)

type SessionEvent struct {
	ID        string
	SessionID string
	UserID    string
	Type      SessionEventType
	Reason    string
	IP        string
	UserAgent string
	Timestamp time.Time
}

func (e *SessionEvent) Validate() error {
	if e.Type == "" {
		return NewValidationError("type", "type is required")
	}

	if e.SessionID == "" {
		return NewValidationError("sessionId", "session_id is required")
	}

	if e.UserID == "" {
		return NewValidationError("userId", "user_id is required")
	}

	if e.Timestamp.IsZero() {
		return NewValidationError("timestamp", "timestamp is required")
	}

	return nil
}

// eventIDBytes is the size in bytes of the random session-event identifier (128 bits).
const eventIDBytes = 16

func NewEventID() (string, error) {
	b := make([]byte, eventIDBytes)

	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate event id: %w", err)
	}

	return base64.URLEncoding.EncodeToString(b), nil
}
