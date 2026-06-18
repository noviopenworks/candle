# mcp-core Specification

## Purpose
TBD - created by archiving change mcp-core-foundation. Update Purpose after archive.
## Requirements
### Requirement: MCP stdio server
The system SHALL run an MCP server over stdio that advertises its tools and resources and responds to standard MCP protocol requests.

#### Scenario: Server advertises tools
- **WHEN** an MCP client connects and sends `initialize` then `tools/list`
- **THEN** the server returns the five base tools (`list_repos`, `resolve_repo`, `query_repo`, `explain_symbol`, `get_file_context`)

### Requirement: Repo listing and resolution tools
The system SHALL provide `list_repos` returning indexed repos with snapshot metadata, and
`resolve_repo` returning the best matching repo for a fuzzy query. When the server is started
with a scope config, listing and resolution SHALL be limited to the configured `(repo, commit)`
snapshots, and resolution SHALL be deterministic: a repo configured with a pinned commit SHALL
always resolve to that snapshot, never to another snapshot of the same repo.

#### Scenario: list_repos returns indexed repos
- **WHEN** `list_repos` is called with at least one indexed repo
- **THEN** it returns each repo with `repo`, `branch`, `commit`, `indexed_at`, and `node_count`

#### Scenario: resolve_repo fuzzy matches
- **WHEN** `resolve_repo` is called with a partial or approximate name
- **THEN** it returns the best matching `org/repo` identity (with candidates when ambiguous)

#### Scenario: Resolution is deterministic when multiple snapshots exist
- **WHEN** the store holds the same repo at more than one commit and the server is started with a
  scope config that pins that repo to one commit
- **THEN** `resolve_repo` and every repo-scoped tool resolve that repo to the pinned snapshot,
  and the other snapshots of that repo are not returned

#### Scenario: list_repos honors the configured scope
- **WHEN** the server is started with a scope config listing a subset of the store's repos
- **THEN** `list_repos` returns only the configured repos at their configured commits, omitting
  all other repos and other-commit snapshots

### Requirement: Structural code navigation tools
The system SHALL provide `query_repo`, `explain_symbol`, and `get_file_context` as deterministic structural queries over the SQLite store, with no natural-language/semantic layer and no runtime dependency on the Graphify CLI.

#### Scenario: query_repo finds a symbol structurally
- **WHEN** `query_repo` is called with `{repo, name}`
- **THEN** it returns matching nodes (and edges when requested) from that repo's index, deterministically

#### Scenario: explain_symbol returns symbol context
- **WHEN** `explain_symbol` is called with a resolvable symbol or node id
- **THEN** it returns the node with its `source_file`, `source_location`, callers, callees, and related edges

#### Scenario: get_file_context returns file symbols
- **WHEN** `get_file_context` is called with a file path in an indexed repo
- **THEN** it returns the symbols defined in that file and their edges

#### Scenario: Unresolved input returns empty, not error
- **WHEN** any tool is called with an unknown repo, symbol, or file
- **THEN** it returns an empty result or a structured "not found", never a crash

### Requirement: Repo and graph resources
The system SHALL expose `repo://org/name` and `graph://org/name/commit/<sha>/...` resources, commit-pinned from manifest metadata and degrading to branch or `latest` when no commit is available.

#### Scenario: repo resource returns snapshot summary
- **WHEN** a client reads `repo://org/name`
- **THEN** it returns that repo's snapshot summary (branch, commit, counts)

#### Scenario: graph resource is commit-pinned when available
- **WHEN** a client reads `graph://org/name/commit/<sha>/node/<node_id>` and the snapshot has that commit
- **THEN** it returns the node JSON for that pinned snapshot; if no commit is recorded, the resource degrades to branch or latest

### Requirement: Config-scoped serving
`candlegraph serve` SHALL accept a scope config (the manifest schema, via the existing `--config`
flag or discovery from the working location). When a scope config is present, the server SHALL
expose only the `(repo, commit)` snapshots declared in it and SHALL omit every other repo and
snapshot in the store from all tools and resources. When no scope config is present, the server
SHALL serve all indexed snapshots (current behavior), making the feature opt-in and backward
compatible.

#### Scenario: Unconfigured repos are omitted everywhere
- **WHEN** the store holds repos A, B, and C and the server is started with a config listing only
  A and B
- **THEN** repo C does not appear in `list_repos`, does not resolve via `resolve_repo`, and every
  repo-scoped tool treats C as not found

#### Scenario: Cross-repo aggregation respects the scope
- **WHEN** `explain_private_library` (or `find_library_consumers` cross-repo aggregation) runs under
  a scope config
- **THEN** it aggregates consumers only among the configured repos, ignoring snapshots outside the
  configured set

#### Scenario: Configured snapshot missing from the store is skipped, not fatal
- **WHEN** the scope config pins a `(repo, commit)` that is not present in the store
- **THEN** the server starts and serves the snapshots that are present, surfacing a warning for the
  missing entry rather than failing

#### Scenario: No config serves everything (backward compatible)
- **WHEN** `serve` is started without a scope config
- **THEN** all indexed snapshots are served exactly as before this change

#### Scenario: Config entry without a commit resolves to the latest snapshot
- **WHEN** a scope config entry names a repo but omits `commit`, and the store holds one or more
  snapshots of that repo
- **THEN** the server resolves that repo to its latest snapshot (by `indexed_at`) deterministically,
  and that single snapshot is the one served for the repo

