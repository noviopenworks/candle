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
