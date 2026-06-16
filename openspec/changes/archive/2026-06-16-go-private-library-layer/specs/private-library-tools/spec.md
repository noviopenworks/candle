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
