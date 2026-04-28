package schema_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/schema"
	schema_mock "github.com/sergeyslonimsky/elara/internal/usecase/schema/mocks"
)

const (
	testJSONSchema   = `{"type": "object", "required": ["host"], "properties": {"host": {"type": "string"}}}`
	validJSONContent = `{"host": "localhost"}`
	invalidContent   = `{"port": 5432}`
)

func TestValidateContentUseCase_NoSchemas(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	lister := schema_mock.NewMockschemaContentLister(ctrl)
	lister.EXPECT().List(gomock.Any(), "ns").Return(nil, nil)

	uc := schema.NewValidateContentUseCase(lister)

	err := uc.Execute(t.Context(), "ns", "/app/config.json", validJSONContent, domain.FormatJSON)

	assert.NoError(t, err)
}

func TestValidateContentUseCase_PatternMiss(t *testing.T) {
	t.Parallel()

	schemas := []*domain.SchemaAttachment{
		{
			ID:          "1",
			Namespace:   "ns",
			PathPattern: "/other/**",
			JSONSchema:  testJSONSchema,
			CreatedAt:   time.Now(),
		},
	}

	ctrl := gomock.NewController(t)
	lister := schema_mock.NewMockschemaContentLister(ctrl)
	lister.EXPECT().List(gomock.Any(), "ns").Return(schemas, nil)

	uc := schema.NewValidateContentUseCase(lister)

	err := uc.Execute(t.Context(), "ns", "/app/config.json", validJSONContent, domain.FormatJSON)

	assert.NoError(t, err)
}

func TestValidateContentUseCase_PatternMatch_ValidContent(t *testing.T) {
	t.Parallel()

	schemas := []*domain.SchemaAttachment{
		{
			ID:          "1",
			Namespace:   "ns",
			PathPattern: "/app/**",
			JSONSchema:  testJSONSchema,
			CreatedAt:   time.Now(),
		},
	}

	ctrl := gomock.NewController(t)
	lister := schema_mock.NewMockschemaContentLister(ctrl)
	lister.EXPECT().List(gomock.Any(), "ns").Return(schemas, nil)

	uc := schema.NewValidateContentUseCase(lister)

	err := uc.Execute(t.Context(), "ns", "/app/config.json", validJSONContent, domain.FormatJSON)

	assert.NoError(t, err)
}

func TestValidateContentUseCase_PatternMatch_InvalidContent(t *testing.T) {
	t.Parallel()

	schemas := []*domain.SchemaAttachment{
		{
			ID:          "1",
			Namespace:   "ns",
			PathPattern: "/app/**",
			JSONSchema:  testJSONSchema,
			CreatedAt:   time.Now(),
		},
	}

	ctrl := gomock.NewController(t)
	lister := schema_mock.NewMockschemaContentLister(ctrl)
	lister.EXPECT().List(gomock.Any(), "ns").Return(schemas, nil)

	uc := schema.NewValidateContentUseCase(lister)

	err := uc.Execute(t.Context(), "ns", "/app/config.json", invalidContent, domain.FormatJSON)

	require.Error(t, err)

	var sve *domain.SchemaValidationError
	require.ErrorAs(t, err, &sve, "expected SchemaValidationError, got %T: %v", err, err)
	assert.NotEmpty(t, sve.Violations)
}

func TestValidateContentUseCase_YAML_Valid(t *testing.T) {
	t.Parallel()

	schemas := []*domain.SchemaAttachment{
		{
			ID:          "1",
			Namespace:   "ns",
			PathPattern: "/app/**",
			JSONSchema:  testJSONSchema,
			CreatedAt:   time.Now(),
		},
	}

	ctrl := gomock.NewController(t)
	lister := schema_mock.NewMockschemaContentLister(ctrl)
	lister.EXPECT().List(gomock.Any(), "ns").Return(schemas, nil)

	uc := schema.NewValidateContentUseCase(lister)

	yamlContent := "host: localhost\n"
	err := uc.Execute(t.Context(), "ns", "/app/config.yaml", yamlContent, domain.FormatYAML)

	assert.NoError(t, err)
}

func TestValidateContentUseCase_FormatOther_Skip(t *testing.T) {
	t.Parallel()

	// The lister should never be called for FormatOther.
	ctrl := gomock.NewController(t)
	lister := schema_mock.NewMockschemaContentLister(ctrl)
	// No EXPECT() calls — lister must not be invoked.
	_ = errors.New("should not be called") // retained for clarity

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
	schemas := []*domain.SchemaAttachment{
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
	}

	ctrl := gomock.NewController(t)
	lister := schema_mock.NewMockschemaContentLister(ctrl)
	lister.EXPECT().List(gomock.Any(), "ns").Return(schemas, nil)

	uc := schema.NewValidateContentUseCase(lister)

	// This content is invalid for /app/** (missing "host") but valid for the exact match.
	// The exact match should win, so no error.
	err := uc.Execute(t.Context(), "ns", "/app/config.json", invalidContent, domain.FormatJSON)

	assert.NoError(t, err)
}
