package config

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const defaultListLimit = 20

//go:generate mockgen -destination=mocks/mock_list.go -package=config_mock . configLister

type configLister interface {
	ListSummariesByPrefix(ctx context.Context, pathPrefix, namespace string) ([]*domain.ConfigSummary, error)
}

type ListParams struct {
	Namespace string
	Path      string
	Limit     int
	Offset    int
	Sort      domain.SortParams
	Query     string // optional: filter entries by name substring
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

type ListUseCase struct {
	configs configLister
}

func NewListUseCase(configs configLister) *ListUseCase {
	return &ListUseCase{configs: configs}
}

func (uc *ListUseCase) Execute(ctx context.Context, params ListParams) (*ListResult, error) {
	path := normalizePath(params.Path)

	prefix := path
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	summaries, err := uc.configs.ListSummariesByPrefix(ctx, prefix, params.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list configs: %w", err)
	}

	entries := buildDirectoryEntries(summaries, prefix, params.Sort)

	// Filter by query if provided.
	if params.Query != "" {
		queryLower := strings.ToLower(params.Query)

		filtered := make([]*DirectoryEntry, 0, len(entries))
		for _, e := range entries {
			if strings.Contains(strings.ToLower(e.Name), queryLower) {
				filtered = append(filtered, e)
			}
		}

		entries = filtered
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	total := len(entries)
	offset := params.Offset

	var paginated []*DirectoryEntry
	if offset < total {
		end := min(offset+limit, total)
		paginated = entries[offset:end]
	}

	return &ListResult{
		Entries: paginated,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

type folderInfo struct {
	childCount int
	latestTime time.Time
}

func buildDirectoryEntries(
	summaries []*domain.ConfigSummary,
	prefix string,
	sortParams domain.SortParams,
) []*DirectoryEntry {
	folders, files := splitSummariesIntoFoldersAndFiles(summaries, prefix)

	// Namespace lock is a property of the listing as a whole; any entry in the
	// scan carries the same value, so sample the first summary.
	var namespaceLocked bool
	if len(summaries) > 0 {
		namespaceLocked = summaries[0].NamespaceLocked
	}

	folderEntries := buildFolderEntries(folders, prefix, namespaceLocked)

	result := make([]*DirectoryEntry, 0, len(folderEntries)+len(files))
	result = append(result, folderEntries...)
	result = append(result, files...)

	sortEntries(result, sortParams)

	return result
}

func splitSummariesIntoFoldersAndFiles(
	summaries []*domain.ConfigSummary,
	prefix string,
) (map[string]*folderInfo, []*DirectoryEntry) {
	folders := make(map[string]*folderInfo)

	var files []*DirectoryEntry

	for _, s := range summaries {
		relative := strings.TrimPrefix(s.Path, prefix)
		if relative == "" {
			continue
		}

		parts := strings.SplitN(relative, "/", 2) //nolint:mnd // split into at most 2 parts: first segment + rest
		name := parts[0]

		if len(parts) > 1 {
			fi, ok := folders[name]
			if !ok {
				fi = &folderInfo{}
				folders[name] = fi
			}

			fi.childCount++

			if s.UpdatedAt.After(fi.latestTime) {
				fi.latestTime = s.UpdatedAt
			}

			continue
		}

		fullPath := prefix + name
		if !strings.HasPrefix(fullPath, "/") {
			fullPath = "/" + fullPath
		}

		files = append(files, &DirectoryEntry{
			Name:            name,
			FullPath:        fullPath,
			IsFile:          true,
			Format:          s.Format,
			Version:         s.Version,
			Locked:          s.Locked,
			NamespaceLocked: s.NamespaceLocked,
			Revision:        s.Revision,
			UpdatedAt:       s.UpdatedAt,
		})
	}

	return folders, files
}

func buildFolderEntries(folders map[string]*folderInfo, prefix string, namespaceLocked bool) []*DirectoryEntry {
	folderEntries := make([]*DirectoryEntry, 0, len(folders))

	for name, fi := range folders {
		fullPath := prefix + name
		if !strings.HasPrefix(fullPath, "/") {
			fullPath = "/" + fullPath
		}

		folderEntries = append(folderEntries, &DirectoryEntry{
			Name:            name,
			FullPath:        fullPath,
			IsFile:          false,
			ChildCount:      fi.childCount,
			UpdatedAt:       fi.latestTime,
			NamespaceLocked: namespaceLocked,
		})
	}

	return folderEntries
}

func sortEntries(entries []*DirectoryEntry, params domain.SortParams) {
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]

		// Folders always before files regardless of sort.
		if a.IsFile != b.IsFile {
			return !a.IsFile
		}

		var less bool

		switch params.Field {
		case "modified":
			less = a.UpdatedAt.Before(b.UpdatedAt)
		default: // "name" or empty
			less = a.Name < b.Name
		}

		if params.Desc {
			return !less
		}

		return less
	})
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return strings.TrimSuffix(path, "/")
}
