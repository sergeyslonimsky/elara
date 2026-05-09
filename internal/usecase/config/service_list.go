package config

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/util/pathutil"
	"github.com/sergeyslonimsky/elara/internal/util/sliceutil"
)

const defaultListLimit = 20

func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	// config/list silently filters: only return results for namespaces the caller can read.
	allowed, _ := s.enforcer.Enforce(claims.Email, params.Namespace, "config", "read")
	if !allowed {
		return &ListResult{
			Entries: nil,
			Total:   0,
			Limit:   params.Limit,
			Offset:  params.Offset,
		}, nil
	}

	path := pathutil.Normalize(params.Path)

	prefix := path
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	summaries, err := s.storage.ListSummariesByPrefix(ctx, prefix, params.Namespace)
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
	paginated := sliceutil.Paginate(entries, offset, limit)

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

		parts := strings.SplitN(relative, "/", 2) //nolint:mnd // path component separation
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
