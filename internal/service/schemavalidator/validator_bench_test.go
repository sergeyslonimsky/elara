package schemavalidator_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/schemavalidator"
)

// Benchmarks for schema validation on the write path. Validate runs on every
// Put into a namespace that has a schema attached, on both the ConnectRPC and
// the etcd-compatible path, so it sits in front of the storage write rather
// than beside it.
//
// The compiled-schema cache is the thing these benchmarks exist to keep
// honest: a hit and a miss differ by orders of magnitude, so a single
// "Validate" number would describe neither. The storage layer is a
// hand-written fake, not a gomock: a mock's own bookkeeping would land in the
// allocation count and swamp what is being measured.

func BenchmarkValidator_Validate_CacheHit_JSON(b *testing.B) {
	b.ReportAllocs()

	v := schemavalidator.New(&benchSchemaStore{schemas: benchSchemas(1)})
	ctx := b.Context()

	// Prime the cache so the compile cost is not attributed to the first
	// measured iteration.
	if err := v.Validate(ctx, "ns", "/svc/app.json", benchValidJSON, domain.FormatJSON); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		if err := v.Validate(ctx, "ns", "/svc/app.json", benchValidJSON, domain.FormatJSON); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidator_Validate_CacheHit_YAML pairs with the JSON benchmark:
// same schema, same cache state, different unmarshaller. The delta is what
// YAML parsing costs on the write path.
func BenchmarkValidator_Validate_CacheHit_YAML(b *testing.B) {
	b.ReportAllocs()

	v := schemavalidator.New(&benchSchemaStore{schemas: benchSchemas(1)})
	ctx := b.Context()

	if err := v.Validate(ctx, "ns", "/svc/app.yaml", benchValidYAML, domain.FormatYAML); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		if err := v.Validate(ctx, "ns", "/svc/app.yaml", benchValidYAML, domain.FormatYAML); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidator_Validate_CacheMiss forces a compile per iteration by
// handing back a distinct schema each time. This is the cost paid on the
// first write after a schema is attached or changed — and the cost that would
// be paid on every write if the cache were ever keyed wrongly.
func BenchmarkValidator_Validate_CacheMiss(b *testing.B) {
	b.ReportAllocs()

	store := &benchSchemaStore{schemas: benchSchemas(b.N), rotate: true}
	v := schemavalidator.New(store)
	ctx := b.Context()

	b.ResetTimer()

	for range b.N {
		if err := v.Validate(ctx, "ns", "/svc/app.json", benchValidJSON, domain.FormatJSON); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidator_Validate_NoSchemaAttached is the common case for a
// namespace nobody has attached a schema to.
//
// It measures the validator's early return only. The fake store answers from
// memory, so the real storage lookup that precedes the early return is NOT in
// this number — that read is a bbolt scan, measured in the storage package.
// What this says is narrower and still worth knowing: once the lookup comes
// back empty, the validator itself adds nothing to the write path.
func BenchmarkValidator_Validate_NoSchemaAttached(b *testing.B) {
	b.ReportAllocs()

	v := schemavalidator.New(&benchSchemaStore{})
	ctx := b.Context()

	b.ResetTimer()

	for b.Loop() {
		if err := v.Validate(ctx, "ns", "/svc/app.json", benchValidJSON, domain.FormatJSON); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidator_Validate_PatternMatching scales the number of attached
// schemas. Every write scores each candidate pattern for specificity, so the
// cost is per attachment in the namespace, not per matching attachment.
func BenchmarkValidator_Validate_PatternMatching(b *testing.B) {
	for _, n := range []int{1, 10, 100} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()

			v := schemavalidator.New(&benchSchemaStore{schemas: benchPatternSchemas(n)})
			ctx := b.Context()

			if err := v.Validate(ctx, "ns", "/svc/app.json", benchValidJSON, domain.FormatJSON); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()

			for b.Loop() {
				if err := v.Validate(ctx, "ns", "/svc/app.json", benchValidJSON, domain.FormatJSON); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkValidator_Validate_Invalid measures the rejection path, which does
// strictly more work than acceptance: the violation tree is walked and
// flattened into a domain error.
func BenchmarkValidator_Validate_Invalid(b *testing.B) {
	b.ReportAllocs()

	v := schemavalidator.New(&benchSchemaStore{schemas: benchSchemas(1)})
	ctx := b.Context()

	if err := v.Validate(ctx, "ns", "/svc/app.json", benchInvalidJSON, domain.FormatJSON); err == nil {
		b.Fatal("expected the invalid payload to be rejected")
	}

	b.ResetTimer()

	for b.Loop() {
		if err := v.Validate(ctx, "ns", "/svc/app.json", benchInvalidJSON, domain.FormatJSON); err == nil {
			b.Fatal("expected the invalid payload to be rejected")
		}
	}
}

const (
	benchValidJSON   = `{"replicas":3,"image":"app:1.2.3","debug":false}`
	benchValidYAML   = "replicas: 3\nimage: \"app:1.2.3\"\ndebug: false\n"
	benchInvalidJSON = `{"replicas":"three","image":"app:1.2.3"}`

	// benchSchemaTemplate carries a %d so each generated copy hashes
	// differently — the cache is keyed on the schema's content hash, so
	// identical bodies would collapse into one entry.
	benchSchemaTemplate = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "bench schema %d",
  "required": ["replicas", "image"],
  "properties": {
    "replicas": {"type": "integer", "minimum": 1, "maximum": 100},
    "image": {"type": "string", "minLength": 1},
    "debug": {"type": "boolean"}
  },
  "additionalProperties": false
}`
)

// benchSchemaStore is a hand-written stand-in for the schema repository.
//
// Not safe for concurrent use: rotate mutates idx without a lock. Every
// benchmark here is sequential, and adding a mutex would put lock traffic
// into the measurement it is meant to stay out of.
type benchSchemaStore struct {
	schemas []*domain.SchemaAttachment
	// rotate hands back one schema per List call instead of the whole slice,
	// advancing through them so each Validate sees content it has not
	// compiled before.
	rotate bool
	idx    int
}

func (s *benchSchemaStore) List(_ context.Context, _ string) ([]*domain.SchemaAttachment, error) {
	if !s.rotate {
		return s.schemas, nil
	}

	cur := s.schemas[s.idx%len(s.schemas)]
	s.idx++

	return []*domain.SchemaAttachment{cur}, nil
}

// benchSchemas builds n attachments that all match /svc/app.json but differ in
// schema content, so no two share a compiled-schema cache entry.
//
// Used by the cache hit and miss benchmarks, which need every returned schema
// to match — a non-matching pattern would make Validate return early and
// compile nothing.
func benchSchemas(n int) []*domain.SchemaAttachment {
	schemas := make([]*domain.SchemaAttachment, 0, n)

	for i := range n {
		schemas = append(schemas, &domain.SchemaAttachment{
			ID:          strconv.Itoa(i),
			Namespace:   "ns",
			PathPattern: "/svc/*",
			JSONSchema:  fmt.Sprintf(benchSchemaTemplate, i),
		})
	}

	return schemas
}

// benchPatternSchemas builds n attachments with n distinct patterns, exactly
// one of which matches /svc/app.json — the shape of a namespace holding
// schemas for several path families, only one of which a given write hits.
//
// The patterns must be distinct or the benchmark lies: identical ones collapse
// into a single pattern-cache entry, and the measurement becomes deduplication
// instead of the per-attachment scoring that actually scales with how much an
// operator has configured.
func benchPatternSchemas(n int) []*domain.SchemaAttachment {
	schemas := make([]*domain.SchemaAttachment, 0, n)

	for i := range n {
		pattern := "/other-" + strconv.Itoa(i) + "/*"
		if i == n-1 {
			pattern = "/svc/*"
		}

		schemas = append(schemas, &domain.SchemaAttachment{
			ID:          strconv.Itoa(i),
			Namespace:   "ns",
			PathPattern: pattern,
			JSONSchema:  fmt.Sprintf(benchSchemaTemplate, i),
		})
	}

	return schemas
}
