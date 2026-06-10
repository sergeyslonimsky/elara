package sessions

import "time"

const (
	defaultWebTTL    = 8 * time.Hour
	defaultCLITTL    = 30 * 24 * time.Hour
	maxWebSlidingTTL = 30 * 24 * time.Hour
	refreshThrottle  = 60 * time.Second
	refreshMinDelta  = 5 * time.Minute
)

// CreateParams holds the inputs required to open a new session.
type CreateParams struct {
	UserID     string
	ClientType string
	IP         string
	UserAgent  string
}

// EventFilter controls which session events are returned by ListEvents.
type EventFilter struct {
	UserID    string
	SessionID string
	Type      string
	From      time.Time
	To        time.Time
	Limit     int
	Offset    int
}
