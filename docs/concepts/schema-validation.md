# Schema Validation

Elara can validate config content against a **JSON Schema** attached to a
namespace (`internal/domain/schema.go`, `internal/usecase/schema/`).

A `SchemaAttachment` binds one JSON Schema to a `(Namespace, PathPattern)` pair.
The pattern is a **glob** (using `github.com/gobwas/glob` with `/` as separator),
so a single schema can cover many configs:

```jsonc
// attach to namespace "prod", pattern "/services/*.json"
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["port"],
  "properties": {
    "port": { "type": "integer", "minimum": 1, "maximum": 65535 }
  }
}
```

**Which schema applies.** When more than one attached pattern matches a config
path, the **most specific pattern wins** — specificity is scored by counting
wildcard characters (`*`, `?`, `[`); fewer wildcards is more specific. On a tie,
the oldest attachment wins (`findBestMatch`). Only the single best match is
applied to a given config.

**When it runs.** Validation happens on every config create and update, before
the write. `other`-format configs are skipped (only `json` and `yaml` are
validated; YAML is re-marshaled through JSON before checking). Schemas are
compiled with `github.com/santhosh-tekuri/jsonschema` v6, which reads the draft
from the schema's `$schema` keyword (through draft 2020-12), and compiled
schemas are cached by content hash.

!!! warning "Not enforced on the etcd-compatible API"
    Schema validation currently runs on writes through the Web UI/ConnectRPC
    `ConfigService`. Writes through the etcd-compatible gRPC API (`Put`) do
    not go through the same validation path yet — see the etcd `Put` entry in
    [Reference → etcd API compatibility](../reference/etcd-compatibility.md).

**On failure.** The write is rejected with a `SchemaValidationError` carrying a
list of violations, each `{path, message, keyword}`. Its message reads:

```
schema validation failed: N violation(s): /field: message [keyword]; ...
```

Over ConnectRPC this maps to an `invalid_argument` error whose message is the
string above, plus a structured `SchemaValidationFailure` error detail
containing every violation's path, message, and keyword
(`internal/handler/v2/errors.go`). See the full mapping in
[Reference → Errors catalog](../reference/errors-catalog.md).
