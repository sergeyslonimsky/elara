package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	configuc "github.com/sergeyslonimsky/elara/internal/usecase/config"
	config_mock "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

func TestListUseCase_DirectoryBrowsing(t *testing.T) {
	t.Parallel()

	now := time.Now()

	summaries := []*domain.ConfigSummary{
		{Path: "/services/api/config.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
		{Path: "/services/api/secrets.yaml", Format: domain.FormatYAML, Version: 2, UpdatedAt: now.Add(-time.Hour)},
		{
			Path:      "/services/web/config.json",
			Format:    domain.FormatJSON,
			Version:   1,
			UpdatedAt: now.Add(-2 * time.Hour),
		},
		{Path: "/databases/pg.json", Format: domain.FormatJSON, Version: 3, UpdatedAt: now},
		{Path: "/config.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
	}

	ctrl := gomock.NewController(t)
	mock := config_mock.NewMockconfigLister(ctrl)
	mock.EXPECT().ListSummariesByPrefix(gomock.Any(), gomock.Any(), gomock.Any()).Return(summaries, nil)

	uc := configuc.NewListUseCase(mock)

	// Root level
	result, err := uc.Execute(t.Context(), configuc.ListParams{
		Namespace: "default",
		Path:      "/",
		Limit:     50,
	})
	require.NoError(t, err)

	// Should have: folders [databases, services] then files [config.json]
	assert.Equal(t, 3, result.Total)
	require.Len(t, result.Entries, 3)

	// Folders first, alphabetical
	assert.Equal(t, "databases", result.Entries[0].Name)
	assert.False(t, result.Entries[0].IsFile)
	assert.Equal(t, 1, result.Entries[0].ChildCount)

	assert.Equal(t, "services", result.Entries[1].Name)
	assert.False(t, result.Entries[1].IsFile)
	assert.Equal(t, 3, result.Entries[1].ChildCount) // 3 files total under services

	// Then files
	assert.Equal(t, "config.json", result.Entries[2].Name)
	assert.True(t, result.Entries[2].IsFile)
	assert.Equal(t, domain.FormatJSON, result.Entries[2].Format)
}

func TestListUseCase_SubfolderBrowsing(t *testing.T) {
	t.Parallel()

	now := time.Now()

	summaries := []*domain.ConfigSummary{
		{Path: "/services/api/config.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
		{Path: "/services/api/secrets.yaml", Format: domain.FormatYAML, Version: 2, UpdatedAt: now},
		{Path: "/services/web/config.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
	}

	ctrl := gomock.NewController(t)
	mock := config_mock.NewMockconfigLister(ctrl)
	mock.EXPECT().ListSummariesByPrefix(gomock.Any(), gomock.Any(), gomock.Any()).Return(summaries, nil)

	uc := configuc.NewListUseCase(mock)

	// /services level
	result, err := uc.Execute(t.Context(), configuc.ListParams{
		Namespace: "default",
		Path:      "/services",
		Limit:     50,
	})
	require.NoError(t, err)

	// Should have: folders [api, web]
	assert.Equal(t, 2, result.Total)
	require.Len(t, result.Entries, 2)

	assert.Equal(t, "api", result.Entries[0].Name)
	assert.Equal(t, "/services/api", result.Entries[0].FullPath)
	assert.False(t, result.Entries[0].IsFile)
	assert.Equal(t, 2, result.Entries[0].ChildCount)

	assert.Equal(t, "web", result.Entries[1].Name)
	assert.Equal(t, "/services/web", result.Entries[1].FullPath)
	assert.False(t, result.Entries[1].IsFile)
	assert.Equal(t, 1, result.Entries[1].ChildCount)
}

func TestListUseCase_LeafFolder(t *testing.T) {
	t.Parallel()

	now := time.Now()

	summaries := []*domain.ConfigSummary{
		{Path: "/services/api/config.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
		{Path: "/services/api/secrets.yaml", Format: domain.FormatYAML, Version: 2, UpdatedAt: now},
	}

	ctrl := gomock.NewController(t)
	mock := config_mock.NewMockconfigLister(ctrl)
	mock.EXPECT().ListSummariesByPrefix(gomock.Any(), gomock.Any(), gomock.Any()).Return(summaries, nil)

	uc := configuc.NewListUseCase(mock)

	result, err := uc.Execute(t.Context(), configuc.ListParams{
		Namespace: "default",
		Path:      "/services/api",
		Limit:     50,
	})
	require.NoError(t, err)

	// Should have only files, sorted alphabetically
	assert.Equal(t, 2, result.Total)
	require.Len(t, result.Entries, 2)

	assert.Equal(t, "config.json", result.Entries[0].Name)
	assert.True(t, result.Entries[0].IsFile)
	assert.Equal(t, domain.FormatJSON, result.Entries[0].Format)

	assert.Equal(t, "secrets.yaml", result.Entries[1].Name)
	assert.True(t, result.Entries[1].IsFile)
	assert.Equal(t, domain.FormatYAML, result.Entries[1].Format)
}

func TestListUseCase_Pagination(t *testing.T) {
	t.Parallel()

	now := time.Now()

	summaries := []*domain.ConfigSummary{
		{Path: "/a/x.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
		{Path: "/b/x.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
		{Path: "/c/x.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
		{Path: "/d.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
		{Path: "/e.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: now},
	}

	ctrl := gomock.NewController(t)
	mock := config_mock.NewMockconfigLister(ctrl)
	mock.EXPECT().ListSummariesByPrefix(gomock.Any(), gomock.Any(), gomock.Any()).Return(summaries, nil).Times(3)

	uc := configuc.NewListUseCase(mock)

	// First page
	result, err := uc.Execute(t.Context(), configuc.ListParams{
		Namespace: "default", Path: "/", Limit: 2, Offset: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, 5, result.Total) // 3 folders + 2 files
	assert.Len(t, result.Entries, 2)
	assert.Equal(t, "a", result.Entries[0].Name) // folder
	assert.Equal(t, "b", result.Entries[1].Name) // folder

	// Second page
	result, err = uc.Execute(t.Context(), configuc.ListParams{
		Namespace: "default", Path: "/", Limit: 2, Offset: 2,
	})
	require.NoError(t, err)
	assert.Len(t, result.Entries, 2)
	assert.Equal(t, "c", result.Entries[0].Name)      // folder
	assert.Equal(t, "d.json", result.Entries[1].Name) // file

	// Third page
	result, err = uc.Execute(t.Context(), configuc.ListParams{
		Namespace: "default", Path: "/", Limit: 2, Offset: 4,
	})
	require.NoError(t, err)
	assert.Len(t, result.Entries, 1)
	assert.Equal(t, "e.json", result.Entries[0].Name) // file
}

func TestListUseCase_EmptyPath(t *testing.T) {
	t.Parallel()

	summaries := []*domain.ConfigSummary{
		{Path: "/test.json", Format: domain.FormatJSON, Version: 1, UpdatedAt: time.Now()},
	}

	ctrl := gomock.NewController(t)
	mock := config_mock.NewMockconfigLister(ctrl)
	mock.EXPECT().ListSummariesByPrefix(gomock.Any(), gomock.Any(), gomock.Any()).Return(summaries, nil)

	uc := configuc.NewListUseCase(mock)

	// Empty path = root
	result, err := uc.Execute(t.Context(), configuc.ListParams{
		Namespace: "default", Path: "", Limit: 50,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "test.json", result.Entries[0].Name)
}
