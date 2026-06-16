## Why

Internal shared Go modules create service relationships that no API spec captures: which repos depend on `git.company.local/platform/auth`, which version each pins, which exported symbols they actually use, and who breaks if an exported interface changes. This change makes private libraries first-class indexed objects — both provider side (what a library exports) and consumer side (who imports it and how).

This is split change **4 of 4** of the MVP. It **depends on `mcp-core-foundation`** and is independent of the OpenAPI and protobuf changes.

## What Changes

- **Go module parser**: parse `go.mod`, `go.sum`, `go.work`, import statements, and exported packages/functions/types/interfaces/constructors.
- **Storage**: `dependencies` (with `ecosystem`, `is_private`), `private_library_exports`, `private_library_usages` tables (index_id-scoped).
- **Two-sided model**: provider record (module → packages → exported symbols) and consumer record (repo → dependency + version → used packages/symbols with file/line).
- **Tools**: `find_private_library` (by name/module-path/purpose), `find_library_consumers` (consumer repos + versions + used symbols).
- **Resources**: `lib://<module-path>[/version/<v>][/package/<p>][/symbol/<s>]`.

## Capabilities

### New Capabilities
- `go-dependency-index`: parse Go module/dependency files and import sites; build provider exports and consumer usages.
- `private-library-tools`: `find_private_library`, `find_library_consumers`, plus `lib://` resources.

### Modified Capabilities
<!-- None: standalone layer over the foundation. -->

## Impact

- Depends on `index_id`/`repo` conventions and the code graph from `mcp-core-foundation`.
- **Private vs. public classification** (matching internal module-path prefixes, e.g. `git.company.local/…`) must be configurable — config lives with the foundation's viper config.
- Used-symbol resolution (which exported symbols a consumer actually references) reuses the code-graph import/reference data; depth of accuracy is a design-phase decision.
- Cross-repo answers require the merged multi-repo graph (same dependency as the protobuf consumer detection).
