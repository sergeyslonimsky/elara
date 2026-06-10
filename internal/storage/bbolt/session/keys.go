package session

import "bytes"

const (
	bucketSessions               = "sessions"
	bucketSessionsByUser         = "sessions_by_user"
	bucketSessionEvents          = "session_events"
	bucketSessionEventsBySession = "session_events_by_session"
	bucketSessionEventsByUser    = "session_events_by_user"

	keySep = byte(0x00)
)

// sessionByUserKey returns the composite key for the sessions_by_user index.
// Format: <userID> + keySep + <sessionID>.
func sessionByUserKey(userID, sessionID string) []byte {
	return append(sessionByUserPrefix(userID), []byte(sessionID)...)
}

func sessionByUserPrefix(userID string) []byte {
	return []byte(userID + string(keySep))
}

func sessionEventBySessionKey(sessionID, eventID string) []byte {
	return append(sessionEventBySessionPrefix(sessionID), []byte(eventID)...)
}

func sessionEventBySessionPrefix(sessionID string) []byte {
	return []byte(sessionID + string(keySep))
}

func sessionEventByUserKey(userID, eventID string) []byte {
	return append(sessionEventByUserPrefix(userID), []byte(eventID)...)
}

func sessionEventByUserPrefix(userID string) []byte {
	return []byte(userID + string(keySep))
}

// hasPrefix reports whether s begins with prefix. Local copy to avoid
// dragging in bytes everywhere callers want a quick check.
func hasPrefix(s, prefix []byte) bool {
	return len(s) >= len(prefix) && bytes.Equal(s[:len(prefix)], prefix)
}
