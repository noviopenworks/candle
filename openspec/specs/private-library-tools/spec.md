# private-library-tools Specification

## Purpose
TBD - created by archiving change go-private-library-layer. Update Purpose after archive.
## Requirements
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

### Requirement: explain_private_library explains a library from both sides
The system SHALL provide `explain_private_library` that, given a fuzzy `query` resolving to
a single internal Go library, returns a both-sides explanation: the provider definition and
the cross-repo consumer aggregation. It SHALL be additive — `find_private_library` and
`find_library_consumers` retain their current behavior. Every response SHALL include an
explicit `limitations` list for deferred behavior.

#### Scenario: Provider and cross-repo consumers in one call
- **WHEN** `explain_private_library` is called with a query resolving to an internal library
  that is defined by one indexed repo and consumed by others
- **THEN** it returns the provider definition (module path, packages, exports, doc synopsis,
  defining repo and commit) together with a consumer list where each entry is a consuming
  repo with its pinned version, used packages, and used symbols (with file and line)

#### Scenario: Unknown query returns not-found
- **WHEN** `explain_private_library` is called with a query that matches no indexed library
  or private dependency
- **THEN** it returns a structured not-found result (`ErrNotFound`), not a crash

### Requirement: Fuzzy resolution with candidate disambiguation
`explain_private_library` SHALL resolve its `query` against private library module paths,
package paths, doc synopsis, and README text (case-insensitive). When exactly one library
matches it SHALL explain that library. When multiple libraries match it SHALL select a
best match and also return the other matches as candidates so the caller can disambiguate.

#### Scenario: Ambiguous query returns best match plus candidates
- **WHEN** the query matches more than one internal library
- **THEN** the result explains the best-match library and lists the remaining matches as
  candidate module paths

### Requirement: Cross-repo consumer aggregation across all indexes
`explain_private_library` SHALL aggregate consumers across all indexed repositories, not a
single index. For a resolved module path it SHALL find every index whose dependencies or
private-library usages reference that module, and report each as a distinct consuming repo
with its pinned version and used symbols. This requirement supersedes the deferred cross-repo
marker that `find_library_consumers` returns, without changing `find_library_consumers`.

#### Scenario: Multiple consuming repos are aggregated
- **WHEN** two or more indexed repos depend on the same private module
- **THEN** the consumer list contains one entry per consuming repo, each with that repo's
  identity, pinned version, and used symbols

#### Scenario: Library consumed without an indexed provider still explains consumers
- **WHEN** the resolved module is consumed by indexed repos but no indexed repo defines it
  (no provider exports)
- **THEN** the provider section is empty and the consumer aggregation is still returned,
  without error

### Requirement: Code-graph linking for exports and usages
`explain_private_library` SHALL link provider exports to provider code-graph nodes and
SHALL best-effort link consumer usages to consumer code-graph nodes. When a link cannot be
resolved, the result SHALL mark that item as unresolved rather than failing the call.

#### Scenario: Export links to a provider code-graph node
- **WHEN** a provider export's symbol matches a code-graph node in the provider repo's index
- **THEN** that export carries a reference to the resolved provider node

#### Scenario: Consumer usage links to the enclosing consumer node
- **WHEN** a consumer usage occurs in a file that has code-graph nodes, and at least one node's
  definition line is at or before the usage line
- **THEN** that usage carries a reference to the node with the greatest definition line at or
  before the usage line (the enclosing definition)

#### Scenario: Unresolved consumer link is marked, not errored
- **WHEN** a consumer usage cannot be matched to a consumer code-graph node
- **THEN** that usage is returned with an explicit unresolved marker and the call still
  succeeds

