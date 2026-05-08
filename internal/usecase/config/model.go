package config

import (
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type ListParams struct {
	Namespace string
	Path      string
	Limit     int
	Offset    int
	Sort      domain.SortParams
	Query     string
}

type DirectoryEntry struct {
	Name            string
	FullPath        string
	IsFile          bool
	Format          domain.Format
	Version         int64
	Revision        int64
	UpdatedAt       time.Time
	ChildCount      int
	Locked          bool
	NamespaceLocked bool
}

type ListResult struct {
	Entries []*DirectoryEntry
	Total   int
	Limit   int
	Offset  int
}

type SearchParams struct {
	Query     string
	Namespace string
	Limit     int
	Offset    int
	Sort      domain.SortParams
}

type SearchResult struct {
	Results []*domain.ConfigSummary
	Total   int
	Limit   int
	Offset  int
}
