package internal_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storageinternal "github.com/sergeyslonimsky/elara/internal/storage/internal"
)

func TestDomainToConfigMeta(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name string
		cfg  *domain.Config
		want storageinternal.ConfigMeta
	}{
		{
			name: "full config",
			cfg: &domain.Config{
				ContentHash:    "hash1",
				Format:         domain.FormatJSON,
				Version:        3,
				Revision:       10,
				CreateRevision: 5,
				Metadata:       map[string]string{"k": "v"},
				Locked:         true,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			want: storageinternal.ConfigMeta{
				ContentHash:    "hash1",
				Format:         "json",
				Version:        3,
				Revision:       10,
				CreateRevision: 5,
				Metadata:       map[string]string{"k": "v"},
				Locked:         true,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
		{
			name: "zero value config",
			cfg:  &domain.Config{},
			want: storageinternal.ConfigMeta{
				Format: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.DomainToConfigMeta(tt.cfg)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfigMetaToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name      string
		meta      storageinternal.ConfigMeta
		content   string
		path      string
		namespace string
		want      *domain.Config
	}{
		{
			name: "known format",
			meta: storageinternal.ConfigMeta{
				ContentHash:    "hash1",
				Format:         "json",
				Version:        3,
				Revision:       10,
				CreateRevision: 5,
				Metadata:       map[string]string{"k": "v"},
				Locked:         true,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			content:   "content",
			path:      "/foo",
			namespace: "default",
			want: &domain.Config{
				Path:           "/foo",
				Content:        "content",
				ContentHash:    "hash1",
				Format:         domain.FormatJSON,
				Version:        3,
				Revision:       10,
				CreateRevision: 5,
				Namespace:      "default",
				Metadata:       map[string]string{"k": "v"},
				Locked:         true,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
		{
			name: "unrecognized format falls back to other",
			meta: storageinternal.ConfigMeta{
				Format: "bogus",
			},
			content:   "content",
			path:      "/foo",
			namespace: "default",
			want: &domain.Config{
				Path:      "/foo",
				Content:   "content",
				Format:    domain.FormatOther,
				Namespace: "default",
				Metadata:  map[string]string{},
			},
		},
		{
			name: "nil metadata becomes empty map",
			meta: storageinternal.ConfigMeta{
				Format:   "yaml",
				Metadata: nil,
			},
			content:   "",
			path:      "/bar",
			namespace: "ns",
			want: &domain.Config{
				Path:      "/bar",
				Content:   "",
				Format:    domain.FormatYAML,
				Namespace: "ns",
				Metadata:  map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.ConfigMetaToDomain(tt.meta, tt.content, tt.path, tt.namespace)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfigMetaToSummary(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name      string
		meta      storageinternal.ConfigMeta
		path      string
		namespace string
		want      *domain.ConfigSummary
	}{
		{
			name: "known format",
			meta: storageinternal.ConfigMeta{
				ContentHash: "hash1",
				Format:      "yaml",
				Version:     2,
				Revision:    7,
				Metadata:    map[string]string{"a": "b"},
				Locked:      false,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			path:      "/p",
			namespace: "ns",
			want: &domain.ConfigSummary{
				Path:        "/p",
				ContentHash: "hash1",
				Format:      domain.FormatYAML,
				Version:     2,
				Revision:    7,
				Namespace:   "ns",
				Metadata:    map[string]string{"a": "b"},
				Locked:      false,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		{
			name: "unrecognized format falls back to other and nil metadata",
			meta: storageinternal.ConfigMeta{
				Format: "nope",
			},
			path:      "/p2",
			namespace: "ns2",
			want: &domain.ConfigSummary{
				Path:      "/p2",
				Format:    domain.FormatOther,
				Namespace: "ns2",
				Metadata:  map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.ConfigMetaToSummary(tt.meta, tt.path, tt.namespace)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestChangelogEntryToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name     string
		entry    storageinternal.ChangelogEntry
		revision int64
		want     *domain.ChangelogEntry
	}{
		{
			name: "full entry",
			entry: storageinternal.ChangelogEntry{
				Type:      int(domain.EventTypeUpdated),
				Path:      "/foo",
				Namespace: "default",
				Version:   4,
				Timestamp: now,
			},
			revision: 42,
			want: &domain.ChangelogEntry{
				Revision:  42,
				Type:      domain.EventTypeUpdated,
				Path:      "/foo",
				Namespace: "default",
				Version:   4,
				Timestamp: now,
			},
		},
		{
			name:     "zero value entry",
			entry:    storageinternal.ChangelogEntry{},
			revision: 0,
			want: &domain.ChangelogEntry{
				Type: domain.EventType(0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.ChangelogEntryToDomain(tt.entry, tt.revision)
			assert.Equal(t, tt.want, got)
		})
	}
}
