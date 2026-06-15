# graph-index Specification

## Purpose
TBD - created by archiving change mcp-core-foundation. Update Purpose after archive.
## Requirements
### Requirement: Repo manifest registration
The system SHALL resolve repos from an explicit viper manifest, where each entry declares a logical `org/repo` identity, the path to that repo's Graphify `graph.json`, and optional `commit` and `branch` metadata. The manifest SHALL be the sole source of which repos exist and where their graphs live.

#### Scenario: Repo declared in manifest is resolvable
- **WHEN** the manifest contains an entry `repo: org/inventory-service` with a valid `graph` path
- **THEN** `org/inventory-service` resolves to an indexed snapshot and appears in `list_repos`

#### Scenario: Commit metadata enables pinned identity
- **WHEN** a manifest entry includes a `commit` SHA
- **THEN** the resulting snapshot records that commit and exposes it for commit-pinned resources

### Requirement: Per-repo graph ingestion into index_id
The system SHALL ingest each repo's own `graph.json` into its own `index_id`, where `index_id` represents one indexed repo snapshot identified by `(repo, commit)`. The system SHALL persist nodes (keyed by Graphify node `id` within the index), edges, and hyperedges. Ingestion SHALL be idempotent: re-indexing the same `(repo, commit)` replaces that snapshot's rows without duplication.

#### Scenario: Graph is ingested into a per-repo index
- **WHEN** `index` runs for a manifest repo with a non-empty `graph.json`
- **THEN** an `index_id` is created for that repo and its nodes/edges/hyperedges are persisted under that `index_id`

#### Scenario: Re-indexing the same commit is idempotent
- **WHEN** `index` runs twice for the same repo and commit
- **THEN** the snapshot's node and edge counts are identical after the second run (no duplicates)

#### Scenario: Cross-repo relations are query-time joins, not merged input
- **WHEN** multiple repos are indexed
- **THEN** each occupies its own `index_id` and cross-repo relationships are computed by joining across indexes; ingesting a single Graphify merged graph is NOT required

### Requirement: Graphify schema tolerance
The system SHALL parse the Graphify `graph.json` node/edge/hyperedge schema and SHALL tolerate missing, empty, or partially malformed graphs without aborting.

#### Scenario: Missing graph file
- **WHEN** a manifest entry points to a non-existent `graph.json`
- **THEN** the system warns and skips that repo without crashing other repos' ingestion

#### Scenario: Empty graph
- **WHEN** a repo's `graph.json` contains no nodes
- **THEN** an empty index is created and the repo is listable with a node count of zero

#### Scenario: Malformed node is skipped
- **WHEN** an individual node or edge is missing required fields
- **THEN** that entry is skipped with a warning and the remaining valid entries are ingested

### Requirement: Cross-index query helper
The system SHALL expose a store-level helper that matches nodes/identifiers across multiple indexes, so downstream layers can compute cross-repo relationships (e.g. `consumed_by`) without materialized cross-repo edges.

#### Scenario: Identifier matched across two repos
- **WHEN** two indexed repos contain nodes sharing a matching identifier
- **THEN** the cross-index helper returns matches spanning both `index_id`s

