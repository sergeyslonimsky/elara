package bbolt

import (
	"log/slog"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

type configMeta struct {
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

type lockHistoryEntry struct {
	Type      int       `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

type changelogEntry struct {
	Type      int       `json:"type"`
	Path      string    `json:"path"`
	Namespace string    `json:"namespace"`
	Version   int64     `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

type namespaceMeta struct {
	Description string    `json:"description"`
	Locked      bool      `json:"locked,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func configMetaToDomain(m *configMeta, content, path, namespace string) *domain.Config {
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

func configMetaToSummary(m *configMeta, path, namespace string) *domain.ConfigSummary {
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

func domainToConfigMeta(cfg *domain.Config) *configMeta {
	return &configMeta{
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

func changelogEntryToDomain(e *changelogEntry, revision int64) *domain.ChangelogEntry {
	return &domain.ChangelogEntry{
		Revision:  revision,
		Type:      domain.EventType(e.Type),
		Path:      e.Path,
		Namespace: e.Namespace,
		Version:   e.Version,
		Timestamp: e.Timestamp,
	}
}

func namespaceMetaToDomain(m *namespaceMeta, name string) *domain.Namespace {
	return &domain.Namespace{
		Name:        name,
		Description: m.Description,
		Locked:      m.Locked,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func domainToNamespaceMeta(ns *domain.Namespace) *namespaceMeta {
	return &namespaceMeta{
		Description: ns.Description,
		Locked:      ns.Locked,
		CreatedAt:   ns.CreatedAt,
		UpdatedAt:   ns.UpdatedAt,
	}
}

type schemaMeta struct {
	ID         string    `json:"id"`
	JSONSchema string    `json:"json_schema"`
	CreatedAt  time.Time `json:"created_at"`
}

func domainToSchemaMeta(s *domain.SchemaAttachment) *schemaMeta {
	return &schemaMeta{
		ID:         s.ID,
		JSONSchema: s.JSONSchema,
		CreatedAt:  s.CreatedAt,
	}
}

func schemaMetaToDomain(m *schemaMeta, namespace, pathPattern string) *domain.SchemaAttachment {
	return &domain.SchemaAttachment{
		ID:          m.ID,
		Namespace:   namespace,
		PathPattern: pathPattern,
		JSONSchema:  m.JSONSchema,
		CreatedAt:   m.CreatedAt,
	}
}
