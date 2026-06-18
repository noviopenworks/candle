# mcp-core Specification

## MODIFIED Requirements

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

## ADDED Requirements

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
