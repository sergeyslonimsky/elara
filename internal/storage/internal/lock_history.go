package internal

import "time"

// LockHistoryEntry is the per-resource lock event row stored in the
// lock_history bucket. Keyed by (namespace, path, seq).
type LockHistoryEntry struct {
	Type      int       `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

// ChangelogEntry is the global changelog row stored in lock_changelog
// (and content changelog) buckets. Keyed by sequence revision.
type ChangelogEntry struct {
	Type      int       `json:"type"`
	Path      string    `json:"path"`
	Namespace string    `json:"namespace"`
	Version   int64     `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}
