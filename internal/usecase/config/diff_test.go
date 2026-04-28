package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
	config_mock "github.com/sergeyslonimsky/elara/internal/usecase/config/mocks"
)

func TestGetDiff_Validation(t *testing.T) {
	t.Parallel()

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

			ctrl := gomock.NewController(t)
			mock := config_mock.NewMockconfigDiffReader(ctrl)
			// No mock expectations: validation fails before any repo call.

			uc := config.NewDiffUseCase(mock)
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

	entry := func(rev int64, content string, ev domain.EventType) *domain.HistoryEntry {
		return &domain.HistoryEntry{Revision: rev, Content: content, EventType: ev}
	}

	tests := []struct {
		name          string
		path          string
		fromRevision  int64
		toRevision    int64
		setupMock     func(mock *config_mock.MockconfigDiffReader, path string)
		wantFromEmpty bool
		wantToEmpty   bool
		wantDiffEmpty bool
		wantDiffHas   string
	}{
		{
			name: "normal case json diff",
			path: "/app.json", fromRevision: 1, toRevision: 3,
			setupMock: func(m *config_mock.MockconfigDiffReader, path string) {
				m.EXPECT().
					GetAtRevision(gomock.Any(), path, "default", int64(3)).
					Return(entry(3, `{"key":"new"}`, domain.EventTypeUpdated), nil)
				m.EXPECT().
					GetAtRevision(gomock.Any(), path, "default", int64(1)).
					Return(entry(1, `{"key":"old"}`, domain.EventTypeCreated), nil)
			},
			wantDiffHas: "-",
		},
		{
			name: "from zero shows all as added",
			path: "/app.json", fromRevision: 0, toRevision: 1,
			setupMock: func(m *config_mock.MockconfigDiffReader, path string) {
				m.EXPECT().
					GetAtRevision(gomock.Any(), path, "default", int64(1)).
					Return(entry(1, `{"key":"val"}`, domain.EventTypeCreated), nil)
			},
			wantFromEmpty: true,
			wantDiffHas:   "+",
		},
		{
			name: "same revision diff empty",
			path: "/app.json", fromRevision: 2, toRevision: 2,
			setupMock: func(m *config_mock.MockconfigDiffReader, path string) {
				e := entry(2, `{"stable":true}`, domain.EventTypeUpdated)
				m.EXPECT().GetAtRevision(gomock.Any(), path, "default", int64(2)).Return(e, nil).Times(2)
			},
			wantDiffEmpty: true,
		},
		{
			name: "yaml whitespace normalized same",
			path: "/config.yaml", fromRevision: 1, toRevision: 2,
			setupMock: func(m *config_mock.MockconfigDiffReader, path string) {
				m.EXPECT().
					GetAtRevision(gomock.Any(), path, "default", int64(2)).
					Return(entry(2, "name: elara\n", domain.EventTypeUpdated), nil)
				m.EXPECT().
					GetAtRevision(gomock.Any(), path, "default", int64(1)).
					Return(entry(1, "name:   elara\n", domain.EventTypeCreated), nil)
			},
			wantDiffEmpty: true,
		},
		{
			name: "json key order normalized same",
			path: "/app.json", fromRevision: 1, toRevision: 2,
			setupMock: func(m *config_mock.MockconfigDiffReader, path string) {
				m.EXPECT().
					GetAtRevision(gomock.Any(), path, "default", int64(2)).
					Return(entry(2, `{"a":1,"b":2}`, domain.EventTypeUpdated), nil)
				m.EXPECT().
					GetAtRevision(gomock.Any(), path, "default", int64(1)).
					Return(entry(1, `{"b":2,"a":1}`, domain.EventTypeCreated), nil)
			},
			wantDiffEmpty: true,
		},
		{
			name: "deleted revision to_content empty",
			path: "/app.json", fromRevision: 4, toRevision: 5,
			setupMock: func(m *config_mock.MockconfigDiffReader, path string) {
				m.EXPECT().
					GetAtRevision(gomock.Any(), path, "default", int64(5)).
					Return(entry(5, "", domain.EventTypeDeleted), nil)
				m.EXPECT().
					GetAtRevision(gomock.Any(), path, "default", int64(4)).
					Return(entry(4, `{"alive":true}`, domain.EventTypeUpdated), nil)
			},
			wantToEmpty: true,
			wantDiffHas: "-",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			mock := config_mock.NewMockconfigDiffReader(ctrl)
			tc.setupMock(mock, tc.path)

			uc := config.NewDiffUseCase(mock)
			result, err := uc.GetDiff(t.Context(), tc.path, "default", tc.fromRevision, tc.toRevision)
			require.NoError(t, err)

			assert.Equal(t, tc.wantFromEmpty, result.FromContent == "")
			assert.Equal(t, tc.wantToEmpty, result.ToContent == "")

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

	ctrl := gomock.NewController(t)
	mock := config_mock.NewMockconfigDiffReader(ctrl)
	mock.EXPECT().GetAtRevision(gomock.Any(), "/app.json", "default", int64(99)).Return(nil, notFoundErr)

	uc := config.NewDiffUseCase(mock)
	_, err := uc.GetDiff(t.Context(), "/app.json", "default", 0, 99)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
