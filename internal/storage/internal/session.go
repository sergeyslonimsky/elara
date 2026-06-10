package internal

import (
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// SessionMeta is the on-disk JSON shape for a domain.Session. The primary
// bucket key is the session ID; a secondary index (`sessions_by_user`) maps
// userID/sessionID composite keys back to the primary bucket.
type SessionMeta struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	ClientType string     `json:"client_type"`
	IP         string     `json:"ip"`
	UserAgent  string     `json:"user_agent"`
	CreatedAt  time.Time  `json:"created_at"`
	LastSeenAt time.Time  `json:"last_seen_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	RevokedBy  string     `json:"revoked_by,omitempty"`
}

func DomainToSessionMeta(s *domain.Session) SessionMeta {
	return SessionMeta{
		ID:         s.ID,
		UserID:     s.UserID,
		ClientType: string(s.ClientType),
		IP:         s.IP,
		UserAgent:  s.UserAgent,
		CreatedAt:  s.CreatedAt,
		LastSeenAt: s.LastSeenAt,
		ExpiresAt:  s.ExpiresAt,
		RevokedAt:  s.RevokedAt,
		RevokedBy:  s.RevokedBy,
	}
}

func SessionMetaToDomain(m SessionMeta) *domain.Session {
	return &domain.Session{
		ID:         m.ID,
		UserID:     m.UserID,
		ClientType: domain.ClientType(m.ClientType),
		IP:         m.IP,
		UserAgent:  m.UserAgent,
		CreatedAt:  m.CreatedAt,
		LastSeenAt: m.LastSeenAt,
		ExpiresAt:  m.ExpiresAt,
		RevokedAt:  m.RevokedAt,
		RevokedBy:  m.RevokedBy,
	}
}

// SessionEventMeta is the on-disk JSON shape for a domain.SessionEvent. The
// primary bucket key is the event ID; two secondary indexes
// (`session_events_by_session`, `session_events_by_user`) hold composite keys
// for prefix scans.
type SessionEventMeta struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Reason    string    `json:"reason,omitempty"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func DomainToSessionEventMeta(e *domain.SessionEvent) SessionEventMeta {
	return SessionEventMeta{
		ID:        e.ID,
		SessionID: e.SessionID,
		UserID:    e.UserID,
		Type:      string(e.Type),
		Reason:    e.Reason,
		IP:        e.IP,
		UserAgent: e.UserAgent,
		Timestamp: e.Timestamp,
	}
}

func SessionEventMetaToDomain(m SessionEventMeta) *domain.SessionEvent {
	return &domain.SessionEvent{
		ID:        m.ID,
		SessionID: m.SessionID,
		UserID:    m.UserID,
		Type:      domain.SessionEventType(m.Type),
		Reason:    m.Reason,
		IP:        m.IP,
		UserAgent: m.UserAgent,
		Timestamp: m.Timestamp,
	}
}
