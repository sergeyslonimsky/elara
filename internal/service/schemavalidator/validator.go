package schemavalidator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gobwas/glob"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

//go:generate mockgen -destination=mocks/schemavalidator_mock.go -package=schemavalidator_mock -source=validator.go

type storage interface {
	List(ctx context.Context, namespace string) ([]*domain.SchemaAttachment, error)
}

// Validator validates config content against attached JSON Schemas.
type Validator struct {
	storage storage
	cache   *compiledSchemaCache
}

// New creates a new Validator.
func New(repo storage) *Validator {
	return &Validator{storage: repo, cache: newCompiledSchemaCache()}
}

// Validate validates the config content against the best-matching JSON Schema for the namespace.
func (s *Validator) Validate(
	ctx context.Context,
	namespace, configPath, content string,
	format domain.Format,
) error {
	// FormatOther is not JSON/YAML — schema validation is not applicable.
	if format == domain.FormatOther {
		return nil
	}

	schemas, err := s.storage.List(ctx, namespace)
	if err != nil {
		return fmt.Errorf("list schemas: %w", err)
	}

	best := findBestMatch(schemas, configPath)
	if best == nil {
		return nil
	}

	jsonValue, err := toJSONValue(content, format)
	if err != nil {
		return fmt.Errorf("convert content to json: %w", err)
	}

	compiled, err := s.compileSchema(best)
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}

	if err := compiled.Validate(jsonValue); err != nil {
		return collectViolations(err)
	}

	return nil
}

func (s *Validator) compileSchema(sa *domain.SchemaAttachment) (*jsonschema.Schema, error) {
	h := sha256.Sum256([]byte(sa.JSONSchema))
	k := cacheKey{namespace: sa.Namespace, pathPattern: sa.PathPattern, schemaHash: hex.EncodeToString(h[:])}

	if compiled, ok := s.cache.get(k); ok {
		return compiled, nil
	}

	compiler := jsonschema.NewCompiler()

	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(sa.JSONSchema))
	if err != nil {
		return nil, fmt.Errorf("unmarshal schema json: %w", err)
	}

	if err := compiler.AddResource("schema.json", doc); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}

	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	s.cache.set(k, compiled)

	return compiled, nil
}

// findBestMatch returns the most specific matching schema (fewest wildcard chars).
// On equal specificity, the oldest CreatedAt wins.
func findBestMatch(schemas []*domain.SchemaAttachment, configPath string) *domain.SchemaAttachment {
	var best *domain.SchemaAttachment
	bestScore := -1

	for _, s := range schemas {
		g, err := glob.Compile(s.PathPattern, '/')
		if err != nil {
			continue
		}

		if !g.Match(configPath) {
			continue
		}

		score := specificity(s.PathPattern)
		if best == nil || score > bestScore || (score == bestScore && s.CreatedAt.Before(best.CreatedAt)) {
			best = s
			bestScore = score
		}
	}

	return best
}

// specificity returns a score inversely proportional to wildcard count.
// Higher score = more specific = better match.
func specificity(pattern string) int {
	wildcards := strings.Count(pattern, "*") + strings.Count(pattern, "?") + strings.Count(pattern, "[")

	return -wildcards
}

func toJSONValue(content string, format domain.Format) (any, error) {
	switch format {
	case domain.FormatJSON:
		var v any
		if err := json.Unmarshal([]byte(content), &v); err != nil {
			return nil, fmt.Errorf("unmarshal json: %w", err)
		}

		return v, nil
	case domain.FormatYAML:
		var v any
		if err := yaml.Unmarshal([]byte(content), &v); err != nil {
			return nil, fmt.Errorf("unmarshal yaml: %w", err)
		}

		// jsonschema/v6 needs JSON-compatible types; re-marshal through JSON.
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("re-marshal yaml as json: %w", err)
		}

		var jv any
		if err := json.Unmarshal(raw, &jv); err != nil {
			return nil, fmt.Errorf("unmarshal re-marshaled json: %w", err)
		}

		return jv, nil
	default:
		return nil, nil //nolint:nilnil // If no known format, return content as is.
	}
}

func collectViolations(err error) *domain.SchemaValidationError {
	violations := []domain.SchemaViolation{}

	if ve, ok := errors.AsType[*jsonschema.ValidationError](err); ok {
		violations = walkViolations(ve, violations)
	}

	if len(violations) == 0 {
		violations = append(violations, domain.SchemaViolation{
			Path:    "",
			Message: err.Error(),
			Keyword: "",
		})
	}

	return &domain.SchemaValidationError{Violations: violations}
}

func walkViolations(ve *jsonschema.ValidationError, acc []domain.SchemaViolation) []domain.SchemaViolation {
	printer := message.NewPrinter(language.English)

	if len(ve.Causes) == 0 {
		path := "/" + strings.Join(ve.InstanceLocation, "/")
		keyword := strings.Join(ve.ErrorKind.KeywordPath(), "/")
		acc = append(acc, domain.SchemaViolation{
			Path:    path,
			Message: ve.ErrorKind.LocalizedString(printer),
			Keyword: keyword,
		})

		return acc
	}

	for _, cause := range ve.Causes {
		acc = walkViolations(cause, acc)
	}

	return acc
}
