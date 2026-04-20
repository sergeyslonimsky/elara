package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

type mockDiffReader struct {
	entries map[int64]*domain.HistoryEntry
	err     error
}

func (m *mockDiffReader) GetAtRevision(_ context.Context, _, _ string, revision int64) (*domain.HistoryEntry, error) {
	if m.err != nil {
		return nil, m.err
	}

	e, ok := m.entries[revision]
	if !ok {
		return nil, domain.NewNotFoundError("revision", "not found")
	}

	return e, nil
}

func TestGetDiff_Validation(t *testing.T) {
	t.Parallel()

	uc := config.NewDiffUseCase(&mockDiffReader{entries: map[int64]*domain.HistoryEntry{}})

	tests := []struct {
		name         string
		path         string
		namespace    string
		fromRevision int64
		toRevision   int64
		wantField    string
	}{
		{
			name: "empty path",
			path: "", namespace: "default", fromRevision: 0, toRevision: 1,
			wantField: "path",
		},
		{
			name: "empty namespace",
			path: "/app.json", namespace: "", fromRevision: 0, toRevision: 1,
			wantField: "namespace",
		},
		{
			name: "to revision zero",
			path: "/app.json", namespace: "default", fromRevision: 0, toRevision: 0,
			wantField: "to_revision",
		},
		{
			name: "from greater than to",
			path: "/app.json", namespace: "default", fromRevision: 5, toRevision: 3,
			wantField: "from_revision",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := uc.GetDiff(t.Context(), tc.path, tc.namespace, tc.fromRevision, tc.toRevision)
			require.Error(t, err)
			var ve *domain.ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Equal(t, tc.wantField, ve.Field)
		})
	}
}

func TestGetDiff_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		path          string
		entries       map[int64]*domain.HistoryEntry
		fromRevision  int64
		toRevision    int64
		wantFromEmpty bool
		wantToEmpty   bool
		wantDiffEmpty bool
		wantDiffHas   string
	}{
		{
			name: "normal case json diff",
			path: "/app.json",
			entries: map[int64]*domain.HistoryEntry{
				1: {Revision: 1, Content: `{"key":"old"}`, EventType: domain.EventTypeCreated},
				3: {Revision: 3, Content: `{"key":"new"}`, EventType: domain.EventTypeUpdated},
			},
			fromRevision: 1, toRevision: 3,
			wantDiffHas: "-",
		},
		{
			name: "from zero shows all as added",
			path: "/app.json",
			entries: map[int64]*domain.HistoryEntry{
				1: {Revision: 1, Content: `{"key":"val"}`, EventType: domain.EventTypeCreated},
			},
			fromRevision: 0, toRevision: 1,
			wantFromEmpty: true,
			wantDiffHas:   "+",
		},
		{
			name: "same revision diff empty",
			path: "/app.json",
			entries: map[int64]*domain.HistoryEntry{
				2: {Revision: 2, Content: `{"stable":true}`, EventType: domain.EventTypeUpdated},
			},
			fromRevision: 2, toRevision: 2,
			wantDiffEmpty: true,
		},
		{
			name: "yaml whitespace normalized same",
			path: "/config.yaml",
			entries: map[int64]*domain.HistoryEntry{
				1: {Revision: 1, Content: "name:   elara\n", EventType: domain.EventTypeCreated},
				2: {Revision: 2, Content: "name: elara\n", EventType: domain.EventTypeUpdated},
			},
			fromRevision: 1, toRevision: 2,
			wantDiffEmpty: true,
		},
		{
			name: "json key order normalized same",
			path: "/app.json",
			entries: map[int64]*domain.HistoryEntry{
				1: {Revision: 1, Content: `{"b":2,"a":1}`, EventType: domain.EventTypeCreated},
				2: {Revision: 2, Content: `{"a":1,"b":2}`, EventType: domain.EventTypeUpdated},
			},
			fromRevision: 1, toRevision: 2,
			wantDiffEmpty: true,
		},
		{
			name: "deleted revision to_content empty",
			path: "/app.json",
			entries: map[int64]*domain.HistoryEntry{
				4: {Revision: 4, Content: `{"alive":true}`, EventType: domain.EventTypeUpdated},
				5: {Revision: 5, Content: "", EventType: domain.EventTypeDeleted},
			},
			fromRevision: 4, toRevision: 5,
			wantToEmpty: true,
			wantDiffHas: "-",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			uc := config.NewDiffUseCase(&mockDiffReader{entries: tc.entries})
			result, err := uc.GetDiff(t.Context(), tc.path, "default", tc.fromRevision, tc.toRevision)
			require.NoError(t, err)

			if tc.wantFromEmpty {
				assert.Empty(t, result.FromContent)
			}

			if tc.wantToEmpty {
				assert.Empty(t, result.ToContent)
			}

			if tc.wantDiffEmpty {
				assert.Empty(t, result.Diff)
			} else if tc.wantDiffHas != "" {
				assert.Contains(t, result.Diff, tc.wantDiffHas)
			}
		})
	}
}

func TestGetDiff_Error_RevisionNotFound(t *testing.T) {
	t.Parallel()

	notFoundErr := domain.NewNotFoundError("revision", "99")
	uc := config.NewDiffUseCase(&mockDiffReader{err: notFoundErr})
	_, err := uc.GetDiff(t.Context(), "/app.json", "default", 0, 99)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
