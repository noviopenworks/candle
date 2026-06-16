# Comet Design Handoff

- Change: go-private-library-layer
- Phase: design
- Mode: compact
- Context hash: 9e56ba455d1d56c0873ac5740f56b640873875ba559e33d7e8f6257127cc1336

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/go-private-library-layer/proposal.md

- Source: openspec/changes/go-private-library-layer/proposal.md
- Lines: 1-29
- SHA256: ca14e0bd29ac21f06c014738cda1b4dfbe7cbe586ef2e4ca86019599a4adcb8e

```md
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
```

## openspec/changes/go-private-library-layer/design.md

- Source: openspec/changes/go-private-library-layer/design.md
- Lines: 1-40
- SHA256: 80d72ea9edbcac354fb8b8f87eebfd44d9df75818b3bbb7d601d4b091675f18c

```md
# Design — go-private-library-layer (high-level)

> Open-phase design: decisions and approach only. Detailed Design Doc + delta specs come in the design phase.

## Architecture

```
 go.mod/sum/work + imports        go-private-library-layer
 ┌──────────────────────┐  parse  ┌────────────────────────────────────────┐
 │ go.mod / go.sum       │ ──────▶ │ dependencies (ecosystem, is_private)    │
 │ go.work / import sites│         │ private_library_exports (provider)      │
 └──────────────────────┘         │ private_library_usages  (consumer)      │
                                  │            │ join                         │
                                  │            ▼                              │
                                  │  code graph (foundation) + merged graph  │
                                  │  tools: find_private_library,            │
                                  │         find_library_consumers           │
                                  │  resources: lib://…                      │
                                  └────────────────────────────────────────┘
```

## Key Decisions

1. **Parse `go.mod`/`go.work` with `golang.org/x/mod/modfile`** (canonical, handles require/replace/exclude). `go.sum` for version verification only.
2. **Exported-symbol extraction via `go/packages` + `go/ast`** (capitalized identifiers in package API). Provider side = the repo defining the module.
3. **Private classification is config-driven**: a list of internal module-path prefixes (viper config shared with the foundation). Everything else is public and not deeply indexed for MVP.
4. **Consumer usage** = imports of a private module + which exported symbols are referenced, with file/line. Resolution depth (full type-check vs. import + identifier heuristic) is a design-phase decision.
5. **Provider↔consumer join** is by module path + version; cross-repo consumer listing uses the merged multi-repo graph.

## Approach Selection

- `find_private_library`: match query against module path / package paths / inferred purpose (from labels/comments).
- `find_library_consumers`: given a module, list repos that `depends_on` it, each with pinned version and `used_symbols`.
- Unused-export detection (exports with no consumer usage) is a natural follow-on but **deferred** unless cheap to include.

## Open Questions (for design phase)

- Used-symbol resolution fidelity (type-checked vs. heuristic) and performance on large dep sets.
- Whether `go.work` multi-module workspaces are in MVP scope.
- Purpose inference for `find_private_library` (labels/doc comments vs. README).
```

## openspec/changes/go-private-library-layer/tasks.md

- Source: openspec/changes/go-private-library-layer/tasks.md
- Lines: 1-43
- SHA256: 4deadde2e6decac86da259cf1a02e77536f1fcc7a28b51ab8f8da359a401d691

```md
# Tasks — go-private-library-layer

> Refined against design doc `docs/superpowers/specs/2026-06-16-go-private-library-layer-design.md`
> and delta specs. Scope: provider exports + consumer usages (go/ast, no build), single-repo
> find_library_consumers; cross-repo consumer aggregation deferred.

## 1. Storage
- [ ] 1.1 Add `dependencies` (ecosystem, is_private, direct), `private_libraries`, `private_library_exports` (with `node_id`), `private_library_usages` tables to `schema.go`
- [ ] 1.2 `internal/store/godep.go`: bundle types + `ReplaceGoDeps` (idempotent per index_id); find/lookup queries

## 2. Dependency parsing
- [ ] 2.1 Add per-repo `go: { modules, private_prefixes }` block to `RepoConfig` (`internal/config`)
- [ ] 2.2 `internal/godep`: parse `go.mod` (require/replace + indirect) and `go.work` (`use`) with `x/mod/modfile`
- [ ] 2.3 Cross-check versions via `go.sum` (mismatch → warning)
- [ ] 2.4 Per-repo private classification by module-path prefix; public deps shallow

## 3. Provider side (exports)
- [ ] 3.1 Extract exported funcs/constructors/types/interfaces/consts/vars via `go/ast` for private modules the repo defines; package doc synopsis + README
- [ ] 3.2 Persist to `private_libraries` + `private_library_exports`

## 4. Consumer side (usages)
- [ ] 4.1 Detect imports of private modules per repo (longest-prefix match → module+version)
- [ ] 4.2 Resolve used symbols via `alias.Symbol` selector scan with file/line → `private_library_usages`

## 5. Graph linking
- [ ] 5.1 `internal/link` export matcher: link each export to a code node by label (package-file scoped when possible); store `node_id`; run in `ingest.Run` after `graph.Load`

## 6. Tools
- [ ] 6.1 `find_private_library` (module/package path, doc synopsis, README match; path-only for provider-less deps)
- [ ] 6.2 `find_library_consumers` (single-repo: version + used packages + used symbols; deferred cross-repo marker)

## 7. Resources
- [ ] 7.1 `lib://<module-path>` + `/version/`, `/package/`, `/symbol/` variants (single-index provider lookup)

## 8. Verification
- [ ] 8.1 Parser unit tests: require/replace/indirect, go.work `use`, go.sum cross-check, export extraction, README/doc synopsis
- [ ] 8.2 Consumer tests: import + selector usage resolution with file/line
- [ ] 8.3 Classification: private deep-indexed, public shallow
- [ ] 8.4 Linker: export → code node match; unmatched → null node_id
- [ ] 8.5 Tool/resource tests: find_private_library, find_library_consumers (deferred marker, not-found), lib:// provider lookup
- [ ] 8.6 Idempotency: re-index → identical row counts

> **Cross-repo consumer aggregation — DEFERRED to a future change** (out of scope; `find_library_consumers` returns a deferred marker for the cross-repo dimension). Not a task in this change's scope.
```

## openspec/changes/go-private-library-layer/specs/go-dependency-index/spec.md

- Source: openspec/changes/go-private-library-layer/specs/go-dependency-index/spec.md
- Lines: 1-71
- SHA256: 34d1e40720a4df5d7c6e9bd3ccef39cb4bfc56a69e4bb298f061559365e11c5a

```md
## ADDED Requirements

### Requirement: Go module discovery via manifest
The system SHALL read the Go module files and private-classification prefixes for a repo from an explicit `go:` block in that repo's manifest entry, containing `modules` (exact `go.mod`/`go.work` file paths) and `private_prefixes` (internal module-path prefixes). The system SHALL NOT auto-discover module files by globbing the filesystem.

#### Scenario: Modules listed in manifest are indexed
- **WHEN** a manifest entry declares `go: { modules: [go.mod], private_prefixes: [git.acme.local/] }` and the file exists
- **THEN** that module's dependencies, exports, and usages are parsed and stored under the repo's `index_id`

#### Scenario: Repo without go block indexes nothing
- **WHEN** a manifest entry has no `go:` block
- **THEN** the repo indexes successfully with zero Go dependency data and no error

### Requirement: Parse Go module files
The system SHALL parse `go.mod` (module path, require with indirect flag, replace) and `go.work` (use directives) with `golang.org/x/mod/modfile`, cross-check require versions against `go.sum`, and persist every dependency tied to the repo's `index_id`.

#### Scenario: Dependencies are recorded with version and ecosystem
- **WHEN** a `go.mod` with require directives is indexed
- **THEN** each required module is stored with its module path, pinned version, `ecosystem` "go", an `is_private` flag, and a `direct` flag

#### Scenario: go.work workspace modules are resolved
- **WHEN** a `go.work` file with `use` directives is indexed
- **THEN** each used module's `go.mod` is parsed and its dependencies are recorded

#### Scenario: go.sum mismatch is a warning
- **WHEN** a required version is missing from or mismatched against `go.sum`
- **THEN** the discrepancy is recorded as a warning and indexing continues

#### Scenario: Malformed module file is tolerated
- **WHEN** a referenced `go.mod`/`go.work` is missing or fails to parse
- **THEN** it is skipped with a warning and the rest of the run continues

### Requirement: Private classification by prefix
The system SHALL classify a module as private if and only if its module path starts with one of the repo's configured `private_prefixes`. Public dependencies SHALL be recorded shallow (path, version, `is_private=false`) and SHALL NOT have exports or usages deeply indexed.

#### Scenario: Private module is classified and deep-indexed
- **WHEN** a dependency's module path matches a configured private prefix
- **THEN** it is stored with `is_private` true and is eligible for export/usage indexing

#### Scenario: Public dependency is shallow
- **WHEN** a dependency's module path matches no private prefix
- **THEN** it is stored with `is_private` false and no exports/usages are extracted for it

### Requirement: Provider export extraction
The system SHALL, for each private module a repo defines, extract its exported top-level symbols (functions, constructors, types, interfaces, consts, vars) via `go/ast`, with each symbol's package import path, kind, and doc comment, plus the package `// Package …` doc synopsis and the module README, and persist them under the repo's `index_id`.

#### Scenario: Exported symbols are indexed
- **WHEN** a repo defines a private module with exported declarations
- **THEN** each exported symbol is stored with its package path, kind, and doc, and the module's README/doc synopsis is stored

#### Scenario: Exported symbols link to code nodes
- **WHEN** an exported symbol matches a code-graph node by name in the same index
- **THEN** the export records that node's id; when no node matches, the export's node id is empty and indexing continues

### Requirement: Consumer usage extraction
The system SHALL, for each repo, detect imports of private modules and the exported symbols actually referenced, via a `go/ast` import + selector heuristic, and persist usages with module path, pinned version, package path, symbol, file, and line.

#### Scenario: Used symbols are resolved with location
- **WHEN** a repo imports a private module and references an exported symbol `pkg.Symbol`
- **THEN** a usage is stored with the module path, version, package path, symbol, file, and line

#### Scenario: Imports without referenced symbols are still recorded
- **WHEN** a repo imports a private module but references no exported symbol in a file
- **THEN** the import (module path, version, package path) is recorded with no symbol

### Requirement: Idempotent Go indexing
The system SHALL make Go indexing idempotent per `index_id`: re-indexing replaces that repo's dependency, library, export, and usage rows without duplication.

#### Scenario: Re-indexing the same repo does not duplicate
- **WHEN** a repo with one module is indexed twice
- **THEN** the dependencies / private_libraries / private_library_exports / private_library_usages counts are identical after the second run
```

## openspec/changes/go-private-library-layer/specs/private-library-tools/spec.md

- Source: openspec/changes/go-private-library-layer/specs/private-library-tools/spec.md
- Lines: 1-38
- SHA256: 42b75b1caa229ccde07660fc69c48b79f6935c214040242650900145c7192052

```md
## ADDED Requirements

### Requirement: find_private_library locates internal libraries
The system SHALL provide `find_private_library` that matches indexed private libraries by name, module path, package path, doc synopsis, or README text (case-insensitive).

#### Scenario: Match by module path or purpose
- **WHEN** `find_private_library` is called with a query matching a private library's module path, a package path, its doc synopsis, or its README
- **THEN** the matching libraries are returned with `{module_path, packages, export_count, doc_synopsis}`

#### Scenario: Consumed module without indexed provider is matchable by path
- **WHEN** a query matches a private dependency module path that has no indexed provider
- **THEN** a path-only entry for that module is returned

### Requirement: find_library_consumers reports single-repo usage
The system SHALL provide `find_library_consumers` that returns, for a `(repo, module_path)`, the queried repo's usage of that module: pinned version, used packages, and used symbols with file and line. Cross-repo consumer aggregation is out of scope for this change; the result SHALL include an explicit deferred marker for the cross-repo dimension.

#### Scenario: Single-repo usage is returned
- **WHEN** `find_library_consumers` is called for a repo that imports the given private module
- **THEN** it returns the pinned version, used packages, used symbols (with file and line), and a deferred marker for cross-repo aggregation

#### Scenario: Cross-repo aggregation is a deferred marker, not an error
- **WHEN** `find_library_consumers` is called for any module
- **THEN** the cross-repo field is an explicit "deferred / not available in this change" marker rather than an error or omitted field

#### Scenario: Unknown module returns not-found
- **WHEN** `find_library_consumers` is called with a module the repo does not depend on
- **THEN** it returns a structured not-found result, not an error/crash

### Requirement: Private library resources
The system SHALL expose `lib://` resources keyed by module path, with optional version, package, and symbol segments, resolving the provider library by a single indexed lookup. A module not defined by an indexed repo SHALL return not-found.

#### Scenario: Library resource returns provider data
- **WHEN** a client reads `lib://<module-path>` for a module defined by an indexed repo
- **THEN** it returns that library's provider data (packages and exports)

#### Scenario: Symbol resource returns the export
- **WHEN** a client reads `lib://<module-path>/symbol/<symbol>` for an indexed export
- **THEN** it returns that export's package path, kind, and doc
```

