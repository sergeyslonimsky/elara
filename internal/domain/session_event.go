package domain

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

type SessionEventType string

const (
	SessionEventCreated        SessionEventType = "created"
	SessionEventRefreshed      SessionEventType = "refreshed"
	SessionEventRevokedByUser  SessionEventType = "revoked_by_user"
	SessionEventRevokedByAdmin SessionEventType = "revoked_by_admin"
	SessionEventRevokedCascade SessionEventType = "revoked_cascade"
	SessionEventExpired        SessionEventType = "expired"
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
