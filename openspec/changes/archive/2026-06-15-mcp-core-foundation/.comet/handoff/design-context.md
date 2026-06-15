# Comet Design Handoff

- Change: mcp-core-foundation
- Phase: design
- Mode: compact
- Context hash: e8c4539c64a611b3095ae07e8b6a594efee3dc06be700e9c54d8de432828e7bc

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/mcp-core-foundation/proposal.md

- Source: openspec/changes/mcp-core-foundation/proposal.md
- Lines: 1-29
- SHA256: 4118ad46d3eb3fc06c3b6c096dff3b75a721a449d4636da81efbcd2f1ed90ee8

```md
## Why

Graphify emits a per-repo `graphify-out/graph.json` (code symbols as nodes, `calls`/`implements`/etc. as edges) and can merge graphs across repos, but it has no served interface an AI coding agent can query, no durable store, and no repo-aware navigation. Every later layer (OpenAPI, protobuf, Go private-library) needs to attach contract/dependency facts to Graphify code symbols and return them over MCP. This change builds that foundation: load Graphify graphs into SQLite and expose them through an MCP stdio server with base navigation tools.

This is split change **1 of 4** in the "API & Private Library Intelligence Layer" MVP. It is a prerequisite for `openapi-contract-layer`, `protobuf-contract-layer`, and `go-private-library-layer`; those three depend on this one and are independent of each other.

## What Changes

- New **Go MCP (stdio) server** with tool + resource dispatch and clean lifecycle.
- **Graphify `graph.json` loader**: ingest a single repo graph and a cross-repo merged graph; normalize nodes/edges into storage; tolerate missing/partial graphs.
- **SQLite storage**: schema for indexed repo snapshots (`index_id` as the unit of one indexed snapshot), code nodes, and edges, designed so later layers hang their contract/dependency tables off `index_id`.
- **Repo registry**: discover/resolve repos and snapshots (the source of the `repo` argument used by every tool) from a Graphify repos directory and/or a config manifest.
- **Base tools**: `list_repos`, `resolve_repo`, `query_repo`, `explain_symbol`, `get_file_context`.
- **Resources**: `repo://…` and `graph://…` (commit-pinned where a SHA is available).

## Capabilities

### New Capabilities
- `graph-index`: ingest Graphify `graph.json` (single + merged) and persist repo snapshots, code nodes, and edges in SQLite; resolve repos/snapshots for tool calls.
- `mcp-core`: the MCP stdio server, base navigation tools (`list_repos`, `resolve_repo`, `query_repo`, `explain_symbol`, `get_file_context`), and `repo://`/`graph://` resources.

### Modified Capabilities
<!-- None: greenfield foundation. -->

## Impact

- New Go module (this repo). Hard external dependency on the **Graphify `graph.json` schema** (`nodes[{id,label,file_type,source_file,…}]`, `edges[{source,target,relation,confidence,…}]`, `hyperedges[]`) — changes there ripple here.
- Defines the storage contract and `repo`/`index_id` conventions every downstream change reuses.
- Stack decision: **Go** (single binary; aligns with the Go dependency layer in change 4).
```

## openspec/changes/mcp-core-foundation/design.md

- Source: openspec/changes/mcp-core-foundation/design.md
- Lines: 1-39
- SHA256: 56a41ef2f72e495ebbe68772fe4beac993c364f3df8e106e06e203bcd503dc8c

```md
# Design — mcp-core-foundation (high-level)

> Open-phase design: architecture decisions and approach selection only. The detailed Design Doc + delta specs are produced in the design phase.

## Architecture

```
 Graphify (external)            mcp-core-foundation (Go)
 ┌────────────────────┐   load  ┌──────────────────────────────────────┐
 │ graph.json (1 repo)│ ──────▶ │  ingest ─▶ SQLite (index_id-scoped)   │
 │ merged graph.json  │         │              nodes / edges / repos     │
 └────────────────────┘         │                    │                   │
                                │   MCP stdio server ─┘                   │
                                │   tools: list_repos, resolve_repo,      │
                                │          query_repo, explain_symbol,    │
                                │          get_file_context               │
                                │   resources: repo://… graph://…         │
                                └──────────────────────────────────────┘
```

## Key Decisions

1. **Consume Graphify output, don't reimplement it.** We read `graph.json`; we never extract. The Graphify node/edge schema is the integration contract.
2. **SQLite as the store**, not Graphify's JSON. JSON is fine for one graph; we need indexed lookups across repos and join targets for later contract/dependency tables. `index_id` = one indexed repo snapshot; every later table references it.
3. **Go**, using an established MCP server library + `mattn/go-sqlite3` (or `modernc.org/sqlite` for cgo-free). Final library choice deferred to design phase. CLI surface (`serve`, `index`, etc.) built with **cobra**; configuration (repo registry paths, private module prefixes for change 4) loaded via **viper**.
4. **Repo identity** is `org/repo`; resolution maps that to a graph snapshot. Source of snapshots (Graphify repos dir vs. explicit manifest) is a **key unknown** — decide in design phase.
5. **Resources are commit-pinned** when a SHA is available; degrade gracefully to branch/latest when Graphify output lacks one.

## Approach Selection

- **Loader**: stream-parse `graph.json`; upsert nodes keyed by Graphify `id`, edges by `(source,target,relation)`. Idempotent re-ingest.
- **`query_repo`**: thin pass-through to graph lookup (symbol/edge search). Whether to shell out to `graphify query` or query SQLite directly is a design-phase decision; default is SQLite-native for self-containment.
- **`explain_symbol` / `get_file_context`**: resolve a node → its edges + `source_file`/`source_location` → return a compact, agent-friendly summary.

## Open Questions (for design phase)

- Repo/snapshot registration model (Graphify repos dir layout vs. config manifest).
- SQLite driver (cgo vs. pure-Go) and MCP Go library selection.
- How much of Graphify's `query` semantics to replicate vs. delegate.
```

## openspec/changes/mcp-core-foundation/tasks.md

- Source: openspec/changes/mcp-core-foundation/tasks.md
- Lines: 1-37
- SHA256: 4a8384ff47d30ac22661320ac5f7f005cbee0848d0293b4f00db05acd4aefec8

```md
# Tasks — mcp-core-foundation

> Open-phase task outline. Refined against the Design Doc + delta specs in the design/build phases.

## 1. Project scaffolding
- [ ] 1.1 Initialize Go module, repo layout (`cmd/`, `internal/`), lint/test tooling
- [ ] 1.2 Choose + wire MCP Go server library; minimal stdio server that lists zero tools

## 2. Storage
- [ ] 2.1 Define SQLite schema: `repos`/`index` (snapshot = `index_id`), `nodes`, `edges`
- [ ] 2.2 Migration/bootstrap on startup; idempotent open

## 3. Graphify ingestion
- [ ] 3.1 Parse `graph.json` (nodes/edges/hyperedges) per the Graphify schema
- [ ] 3.2 Loader: upsert nodes by Graphify `id`, edges by `(source,target,relation)`; idempotent re-ingest
- [ ] 3.3 Support cross-repo merged graph input
- [ ] 3.4 Tolerate missing/empty/partial graphs without erroring

## 4. Repo registry / resolution
- [ ] 4.1 Decide + implement repo-snapshot discovery (resolves the design-phase unknown)
- [ ] 4.2 Map `org/repo` (+ branch/commit) → indexed snapshot

## 5. Base MCP tools
- [ ] 5.1 `list_repos`
- [ ] 5.2 `resolve_repo`
- [ ] 5.3 `query_repo`
- [ ] 5.4 `explain_symbol`
- [ ] 5.5 `get_file_context`

## 6. Resources
- [ ] 6.1 `repo://…` resource handler
- [ ] 6.2 `graph://…` resource handler (commit-pinned, graceful degrade)

## 7. Verification
- [ ] 7.1 Ingest a sample repo graph; tools return expected results
- [ ] 7.2 Empty/no-graph repo indexes cleanly and tools return empty (not errors)
- [ ] 7.3 End-to-end: server starts over stdio, advertises tools, responds to a query
```

## openspec/changes/mcp-core-foundation/specs/graph-index/spec.md

- Source: openspec/changes/mcp-core-foundation/specs/graph-index/spec.md
- Lines: 1-49
- SHA256: 786d9a7bcd40d02a48f01b32705d4be4d707a2835f04cc7baa3ba781f4c3feff

```md
## ADDED Requirements

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
```

## openspec/changes/mcp-core-foundation/specs/mcp-core/spec.md

- Source: openspec/changes/mcp-core-foundation/specs/mcp-core/spec.md
- Lines: 1-49
- SHA256: d137da8dea6696cdda7c73ba1837d4d46cd79458e10dd99cb05272d26a317fbd

```md
## ADDED Requirements

### Requirement: MCP stdio server
The system SHALL run an MCP server over stdio that advertises its tools and resources and responds to standard MCP protocol requests.

#### Scenario: Server advertises tools
- **WHEN** an MCP client connects and sends `initialize` then `tools/list`
- **THEN** the server returns the five base tools (`list_repos`, `resolve_repo`, `query_repo`, `explain_symbol`, `get_file_context`)

### Requirement: Repo listing and resolution tools
The system SHALL provide `list_repos` returning indexed repos with snapshot metadata, and `resolve_repo` returning the best matching repo for a fuzzy query.

#### Scenario: list_repos returns indexed repos
- **WHEN** `list_repos` is called with at least one indexed repo
- **THEN** it returns each repo with `repo`, `branch`, `commit`, `indexed_at`, and `node_count`

#### Scenario: resolve_repo fuzzy matches
- **WHEN** `resolve_repo` is called with a partial or approximate name
- **THEN** it returns the best matching `org/repo` identity (with candidates when ambiguous)

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
```

