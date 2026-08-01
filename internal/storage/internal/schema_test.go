package internal_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeyslonimsky/elara/internal/domain"
	storageinternal "github.com/sergeyslonimsky/elara/internal/storage/internal"
)

func TestDomainToSchemaMeta(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name   string
		schema *domain.SchemaAttachment
		want   storageinternal.SchemaMeta
	}{
		{
			name: "full schema",
			schema: &domain.SchemaAttachment{
				ID:          "schema-1",
				Namespace:   "default",
				PathPattern: "/foo",
				JSONSchema:  `{"type":"object"}`,
				CreatedAt:   now,
			},
			want: storageinternal.SchemaMeta{
				ID:         "schema-1",
				JSONSchema: `{"type":"object"}`,
				CreatedAt:  now,
			},
		},
		{
			name:   "zero value schema",
			schema: &domain.SchemaAttachment{},
			want:   storageinternal.SchemaMeta{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.DomainToSchemaMeta(tt.schema)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSchemaMetaToDomain(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name        string
		meta        storageinternal.SchemaMeta
		namespace   string
		pathPattern string
		want        *domain.SchemaAttachment
	}{
		{
			name: "full meta",
			meta: storageinternal.SchemaMeta{
				ID:         "schema-1",
				JSONSchema: `{"type":"object"}`,
				CreatedAt:  now,
			},
			namespace:   "default",
			pathPattern: "/foo/*",
			want: &domain.SchemaAttachment{
				ID:          "schema-1",
				Namespace:   "default",
				PathPattern: "/foo/*",
				JSONSchema:  `{"type":"object"}`,
				CreatedAt:   now,
			},
		},
		{
			name:        "zero value meta",
			meta:        storageinternal.SchemaMeta{},
			namespace:   "ns",
			pathPattern: "/p",
			want: &domain.SchemaAttachment{
				Namespace:   "ns",
				PathPattern: "/p",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := storageinternal.SchemaMetaToDomain(tt.meta, tt.namespace, tt.pathPattern)
			assert.Equal(t, tt.want, got)
		})
	}
}
