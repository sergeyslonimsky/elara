package domain

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

type ClientType string

const (
	ClientTypeWeb ClientType = "web"
	ClientTypeCLI ClientType = "cli"
)

type Session struct {
	ID         string
	UserID     string
	ClientType ClientType
	IP         string
	UserAgent  string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	RevokedBy  string
}

func (s *Session) IsActive() bool {
	return s.RevokedAt == nil
}

func (s *Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

func (s *Session) EnsureActive(now time.Time) error {
	if !s.IsActive() {
		return ErrSessionRevoked
	}

	if s.IsExpired(now) {
		return ErrSessionExpired
	}

	return nil
}

// sessionIDBytes is the size in bytes of the random session identifier (256 bits).
const sessionIDBytes = 32

// NewSessionID generates a cryptographically random 256-bit session identifier,
// base64 URL-encoded. Never use math/rand for session identifiers.
func NewSessionID() (string, error) {
	b := make([]byte, sessionIDBytes)

	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}

	return base64.URLEncoding.EncodeToString(b), nil
}
