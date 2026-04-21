package domain

import "time"

type EventType int

const (
	EventTypeCreated EventType = iota + 1
	EventTypeUpdated
	EventTypeDeleted
	EventTypeLocked
	EventTypeUnlocked
	EventTypeNamespaceLocked
	EventTypeNamespaceUnlocked
)

func (e EventType) String() string {
	switch e {
	case EventTypeCreated:
		return "CREATED"
	case EventTypeUpdated:
		return "UPDATED"
	case EventTypeDeleted:
		return "DELETED"
	case EventTypeLocked:
		return "LOCKED"
	case EventTypeUnlocked:
		return "UNLOCKED"
	case EventTypeNamespaceLocked:
		return "NAMESPACE_LOCKED"
	case EventTypeNamespaceUnlocked:
		return "NAMESPACE_UNLOCKED"
	default:
		return "UNKNOWN"
	}
}

type WatchEvent struct {
	Type      EventType
	Path      string
	Namespace string
	Revision  int64 // mutation revision; for deletes where Config is nil, this is the delete revision
	Config    *Config
	Timestamp time.Time
}

type ChangelogEntry struct {
	Revision  int64
	Type      EventType
	Path      string
	Namespace string
	Version   int64
	Timestamp time.Time
}

type HistoryEntry struct {
	Revision    int64
	Content     string
	ContentHash string
	EventType   EventType
	Timestamp   time.Time
}

type ConfigDiff struct {
	FromRevision int64
	ToRevision   int64
	FromContent  string
	ToContent    string
	Diff         string
}
