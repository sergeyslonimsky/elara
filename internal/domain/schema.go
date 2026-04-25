package domain

import (
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type SchemaAttachment struct {
	ID          string
	Namespace   string
	PathPattern string
	JSONSchema  string
	CreatedAt   time.Time
}

type SchemaKey struct {
	Namespace   string
	PathPattern string
}

func (s *SchemaAttachment) Key() SchemaKey {
	return SchemaKey{Namespace: s.Namespace, PathPattern: s.PathPattern}
}

// ValidateJSONSchema validates that the given string is a valid JSON Schema.
func ValidateJSONSchema(schema string) error {
	compiler := jsonschema.NewCompiler()

	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(schema))
	if err != nil {
		return NewValidationError("json_schema", "invalid JSON: "+err.Error())
	}

	if err := compiler.AddResource("schema.json", doc); err != nil {
		return NewValidationError("json_schema", "failed to add schema resource: "+err.Error())
	}

	if _, err := compiler.Compile("schema.json"); err != nil {
		return NewValidationError("json_schema", "invalid JSON Schema: "+err.Error())
	}

	return nil
}
