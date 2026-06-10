package internal

import (
	"log/slog"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// ConfigMeta is the on-disk JSON shape for a domain.Config metadata record
// stored in the `meta` bucket. The bucket key encodes (namespace, path) — so
// neither Namespace nor Path is serialized in the value.
type ConfigMeta struct {
	ContentHash    string            `json:"content_hash"`
	Format         string            `json:"format"`
	Version        int64             `json:"version"`
	Revision       int64             `json:"revision"`
	CreateRevision int64             `json:"create_revision"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Locked         bool              `json:"locked,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

func DomainToConfigMeta(cfg *domain.Config) ConfigMeta {
	return ConfigMeta{
		ContentHash:    cfg.ContentHash,
		Format:         cfg.Format.String(),
		Version:        cfg.Version,
		Revision:       cfg.Revision,
		CreateRevision: cfg.CreateRevision,
		Metadata:       cfg.Metadata,
		Locked:         cfg.Locked,
		CreatedAt:      cfg.CreatedAt,
		UpdatedAt:      cfg.UpdatedAt,
	}
}

func ConfigMetaToDomain(m ConfigMeta, content, path, namespace string) *domain.Config {
	format, err := domain.ParseFormat(m.Format)
	if err != nil {
		slog.Warn("unrecognized format in stored metadata", "format", m.Format)
		format = domain.FormatOther
	}

	metadata := m.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}

	return &domain.Config{
		Path:           path,
		Content:        content,
		ContentHash:    m.ContentHash,
		Format:         format,
		Version:        m.Version,
		Revision:       m.Revision,
		CreateRevision: m.CreateRevision,
		Namespace:      namespace,
		Metadata:       metadata,
		Locked:         m.Locked,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func ConfigMetaToSummary(m ConfigMeta, path, namespace string) *domain.ConfigSummary {
	format, err := domain.ParseFormat(m.Format)
	if err != nil {
		slog.Warn("unrecognized format in stored metadata", "format", m.Format)
		format = domain.FormatOther
	}

	metadata := m.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}

	return &domain.ConfigSummary{
		Path:        path,
		ContentHash: m.ContentHash,
		Format:      format,
		Version:     m.Version,
		Revision:    m.Revision,
		Namespace:   namespace,
		Metadata:    metadata,
		Locked:      m.Locked,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func ChangelogEntryToDomain(e ChangelogEntry, revision int64) *domain.ChangelogEntry {
	return &domain.ChangelogEntry{
		Revision:  revision,
		Type:      domain.EventType(e.Type),
		Path:      e.Path,
		Namespace: e.Namespace,
		Version:   e.Version,
		Timestamp: e.Timestamp,
	}
}
