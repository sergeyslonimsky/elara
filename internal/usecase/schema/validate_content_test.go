package schema_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
)

const (
	testJSONSchema   = `{"type": "object", "required": ["host"], "properties": {"host": {"type": "string"}}}`
	validJSONContent = `{"host": "localhost"}`
	invalidContent   = `{"port": 5432}`
)

type mockSchemaLister struct {
	schemas []*domain.SchemaAttachment
	err     error
}

func (m *mockSchemaLister) List(_ context.Context, _ string) ([]*domain.SchemaAttachment, error) {
	return m.schemas, m.err
}

func TestValidateContentUseCase_NoSchemas(t *testing.T) {
	t.Parallel()

	uc := schema.NewValidateContentUseCase(&mockSchemaLister{schemas: nil})

	err := uc.Execute(t.Context(), "ns", "/app/config.json", validJSONContent, domain.FormatJSON)

	assert.NoError(t, err)
}

func TestValidateContentUseCase_PatternMiss(t *testing.T) {
	t.Parallel()

	lister := &mockSchemaLister{
		schemas: []*domain.SchemaAttachment{
			{
				ID:          "1",
				Namespace:   "ns",
				PathPattern: "/other/**",
				JSONSchema:  testJSONSchema,
				CreatedAt:   time.Now(),
			},
		},
	}

	uc := schema.NewValidateContentUseCase(lister)

	err := uc.Execute(t.Context(), "ns", "/app/config.json", validJSONContent, domain.FormatJSON)

	assert.NoError(t, err)
}

func TestValidateContentUseCase_PatternMatch_ValidContent(t *testing.T) {
	t.Parallel()

	lister := &mockSchemaLister{
		schemas: []*domain.SchemaAttachment{
			{
				ID:          "1",
				Namespace:   "ns",
				PathPattern: "/app/**",
				JSONSchema:  testJSONSchema,
				CreatedAt:   time.Now(),
			},
		},
	}

	uc := schema.NewValidateContentUseCase(lister)

	err := uc.Execute(t.Context(), "ns", "/app/config.json", validJSONContent, domain.FormatJSON)

	assert.NoError(t, err)
}

func TestValidateContentUseCase_PatternMatch_InvalidContent(t *testing.T) {
	t.Parallel()

	lister := &mockSchemaLister{
		schemas: []*domain.SchemaAttachment{
			{
				ID:          "1",
				Namespace:   "ns",
				PathPattern: "/app/**",
				JSONSchema:  testJSONSchema,
				CreatedAt:   time.Now(),
			},
		},
	}

	uc := schema.NewValidateContentUseCase(lister)

	err := uc.Execute(t.Context(), "ns", "/app/config.json", invalidContent, domain.FormatJSON)

	require.Error(t, err)

	var sve *domain.SchemaValidationError
	require.ErrorAs(t, err, &sve, "expected SchemaValidationError, got %T: %v", err, err)
	assert.NotEmpty(t, sve.Violations)
}

func TestValidateContentUseCase_YAML_Valid(t *testing.T) {
	t.Parallel()

	lister := &mockSchemaLister{
		schemas: []*domain.SchemaAttachment{
			{
				ID:          "1",
				Namespace:   "ns",
				PathPattern: "/app/**",
				JSONSchema:  testJSONSchema,
				CreatedAt:   time.Now(),
			},
		},
	}

	uc := schema.NewValidateContentUseCase(lister)

	yamlContent := "host: localhost\n"
	err := uc.Execute(t.Context(), "ns", "/app/config.yaml", yamlContent, domain.FormatYAML)

	assert.NoError(t, err)
}

func TestValidateContentUseCase_FormatOther_Skip(t *testing.T) {
	t.Parallel()

	// The lister should never be called for FormatOther.
	lister := &mockSchemaLister{err: errors.New("should not be called")}

	uc := schema.NewValidateContentUseCase(lister)

	err := uc.Execute(t.Context(), "ns", "/app/config.txt", "any content", domain.FormatOther)

	assert.NoError(t, err)
}

func TestValidateContentUseCase_MostSpecificPatternWins(t *testing.T) {
	t.Parallel()

	// The broader schema requires a "host" field.
	// The exact schema allows any object (no required fields).
	exactSchema := `{"type": "object", "properties": {"port": {"type": "integer"}}}`

	createdAt := time.Now()
	lister := &mockSchemaLister{
		schemas: []*domain.SchemaAttachment{
			{
				ID:          "1",
				Namespace:   "ns",
				PathPattern: "/app/**",
				JSONSchema:  testJSONSchema,
				CreatedAt:   createdAt,
			},
			{
				ID:          "2",
				Namespace:   "ns",
				PathPattern: "/app/config.json",
				JSONSchema:  exactSchema,
				CreatedAt:   createdAt,
			},
		},
	}

	uc := schema.NewValidateContentUseCase(lister)

	// This content is invalid for /app/** (missing "host") but valid for the exact match.
	// The exact match should win, so no error.
	err := uc.Execute(t.Context(), "ns", "/app/config.json", invalidContent, domain.FormatJSON)

	assert.NoError(t, err)
}
