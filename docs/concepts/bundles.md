# Bundles

A **bundle** is a portable snapshot of configs used for export/import
(`internal/domain/bundle.go`, `internal/usecase/transfer/`).

- `BundleConfig` — `{Path, Content, Format, Metadata}`.
- `NamespaceBundle` — one namespace and its configs.
- `AllBundle` — every namespace (multi-namespace export/import).

**Export** produces JSON or YAML (encoding chosen per request), optionally
packaged as a **ZIP** for an all-namespaces export (one file per namespace under
`namespaces/<name>.<ext>` plus an index).

**Import** replays a bundle. When a config in the bundle already exists, the
`onConflict` mode decides the outcome (`service_import.go`):

| Mode | Behavior on an existing config |
|------|-------------------------------|
| `SKIP` (default, also used for `UNSPECIFIED`) | Left untouched; counted as *Skipped*. |
| `OVERWRITE` | Existing config is updated (its current `Version` is preserved); counted as *Updated*. |
| `FAIL` | Recorded as an error (`"config already exists"`); counted as *Failed*. |

Configs that don't yet exist are always created. Imports can target an explicit
namespace (overriding the bundle's own namespace field) or infer scope from the
bundle; a multi-namespace `AllBundle` import requires `Transfer:write` on `*`.

**Preview is dry-run, not a diff.** The import preview runs with `dryRun = true`
and returns an `ImportReport` — aggregate counts of `Created` / `Updated` /
`Skipped` / `Failed` plus a per-path error list. It does **not** show a
per-config content diff: an `OVERWRITE` preview reports a config as *Updated*
without telling you what would change. Treat the preview as a summary of how
many entries fall into each bucket, not as a review of the actual changes.
