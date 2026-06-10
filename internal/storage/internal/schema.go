package internal

import (
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// SchemaMeta is the on-disk JSON shape for a domain.SchemaAttachment. The
// bucket key encodes (namespace, pathPattern) so those fields are NOT
// serialized in the value.
type SchemaMeta struct {
	ID         string    `json:"id"`
	JSONSchema string    `json:"json_schema"`
	CreatedAt  time.Time `json:"created_at"`
}

func DomainToSchemaMeta(s *domain.SchemaAttachment) SchemaMeta {
	return SchemaMeta{
		ID:         s.ID,
		JSONSchema: s.JSONSchema,
		CreatedAt:  s.CreatedAt,
	}
}

func SchemaMetaToDomain(m SchemaMeta, namespace, pathPattern string) *domain.SchemaAttachment {
	return &domain.SchemaAttachment{
		ID:          m.ID,
		Namespace:   namespace,
		PathPattern: pathPattern,
		JSONSchema:  m.JSONSchema,
		CreatedAt:   m.CreatedAt,
	}
}
