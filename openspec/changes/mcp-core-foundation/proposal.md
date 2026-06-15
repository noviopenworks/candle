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
