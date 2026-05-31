package bbolt

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type sessionMeta struct {
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

func domainToSessionMeta(s *domain.Session) *sessionMeta {
	return &sessionMeta{
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

func sessionMetaToDomain(m *sessionMeta) *domain.Session {
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

func sessionMetaFromBytes(data []byte) (*sessionMeta, error) {
	var m sessionMeta

	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &m, nil
}

type sessionEventMeta struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Reason    string    `json:"reason,omitempty"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func domainToSessionEventMeta(e *domain.SessionEvent) *sessionEventMeta {
	return &sessionEventMeta{
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

func sessionEventMetaToDomain(m *sessionEventMeta) *domain.SessionEvent {
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

func sessionEventMetaFromBytes(data []byte) (*sessionEventMeta, error) {
	var m sessionEventMeta

	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal session event: %w", err)
	}

	return &m, nil
}
