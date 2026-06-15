---
comet_change: mcp-core-foundation
role: technical-design
canonical_spec: openspec
archived-with: 2026-06-15-mcp-core-foundation
status: final
---

# Technical Design — mcp-core-foundation

Foundation for the API & Private Library Intelligence Layer: a Go MCP (stdio) server that
ingests Graphify `graph.json` into SQLite and exposes base code-navigation tools. Three
later changes (`openapi-contract-layer`, `protobuf-contract-layer`, `go-private-library-layer`)
hang their contract/dependency tables off the `index_id` and tool conventions defined here.

> Canonical capability specs live in OpenSpec delta specs:
> `openspec/changes/mcp-core-foundation/specs/{graph-index,mcp-core}/spec.md`.
> This document is the technical design, not the requirements source of truth.

## Stack decisions

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Language | Go | Single static binary; aligns with the Go dependency layer (change 4). |
| SQLite driver | `modernc.org/sqlite` (pure Go) | No cgo → trivial cross-compile, single binary; adequate for read-heavy graph queries. |
| MCP server | official `modelcontextprotocol/go-sdk` | Spec alignment. Fallback `mark3labs/mcp-go` behind the same internal interface if the SDK proves unstable. |
| CLI | cobra | `serve` / `index` subcommands. |
| Config | viper | Repo manifest + (later) private-module prefixes. |

## Repo layout

```
cmd/main.go            cobra root + subcommands: serve, index
internal/
  config/    viper manifest loading
  registry/  org/repo (+branch/commit) → index_id resolution; fuzzy match
  store/     sqlite open/migrate + structural queries (incl. cross-index helper)
  graph/     graph.json parser + idempotent loader
  mcp/       official Go SDK server wiring
    tools/   list_repos, resolve_repo, query_repo, explain_symbol, get_file_context
    resources/  repo://, graph://
```

## Configuration (viper manifest)

```yaml
repos:
  - repo: org/inventory-service      # logical identity; the `repo` arg in every tool
    graph: /abs/path/graphify-out/graph.json
    commit: abc123                   # optional; enables commit-pinned resources
    branch: main                     # optional; fallback pin
```

The manifest is the single source of which repos exist and where each repo's `graph.json`
lives. It is explicit (not auto-discovered) so resources can be genuinely commit-pinned —
Graphify's `graph.json` does not reliably carry a commit SHA.

## Integration contract: Graphify `graph.json`

Hard external dependency. The loader reads:

- `nodes[]`: `{id, label, file_type(code|document|paper|image|rationale|concept), source_file, source_location, source_url, captured_at, author, contributor}`
- `edges[]`: `{source, target, relation(calls|implements|references|cites|conceptually_related_to|shares_data_with|semantically_similar_to|rationale_for), confidence(EXTRACTED|INFERRED|AMBIGUOUS), confidence_score, source_file, source_location, weight}`
- `hyperedges[]`: `{id, label, nodes[], relation, confidence, confidence_score, source_file}`

Schema drift in Graphify ripples here; the loader validates required fields and skips
malformed entries with a warning rather than aborting.

## Data model (SQLite)

`index_id` is the unit of one indexed repo snapshot. **Every downstream contract/dependency
table references `index_id`.**

```sql
CREATE TABLE repos (
  id    INTEGER PRIMARY KEY,
  org   TEXT NOT NULL,
  name  TEXT NOT NULL,
  UNIQUE(org, name)
);

CREATE TABLE indexes (              -- id == index_id
  id          INTEGER PRIMARY KEY,
  repo_id     INTEGER NOT NULL REFERENCES repos(id),
  commit_sha  TEXT,
  branch      TEXT,
  graph_path  TEXT NOT NULL,
  ingested_at TEXT NOT NULL,
  UNIQUE(repo_id, commit_sha)
);

CREATE TABLE nodes (
  index_id        INTEGER NOT NULL REFERENCES indexes(id),
  node_id         TEXT NOT NULL,     -- Graphify node id
  label           TEXT,
  file_type       TEXT,
  source_file     TEXT,
  source_location TEXT,
  source_url      TEXT,
  captured_at     TEXT,
  author          TEXT,
  contributor     TEXT,
  PRIMARY KEY (index_id, node_id)
);

CREATE TABLE edges (
  index_id         INTEGER NOT NULL REFERENCES indexes(id),
  source           TEXT NOT NULL,
  target           TEXT NOT NULL,
  relation         TEXT NOT NULL,
  confidence       TEXT,
  confidence_score REAL,
  weight           REAL,
  source_file      TEXT
);

CREATE TABLE hyperedges (
  index_id         INTEGER NOT NULL REFERENCES indexes(id),
  hyperedge_id     TEXT NOT NULL,
  label            TEXT,
  relation         TEXT,
  confidence       TEXT,
  confidence_score REAL,
  source_file      TEXT,
  PRIMARY KEY (index_id, hyperedge_id)
);

CREATE TABLE hyperedge_members (
  index_id     INTEGER NOT NULL,
  hyperedge_id TEXT NOT NULL,
  node_id      TEXT NOT NULL
);

CREATE INDEX idx_nodes_label   ON nodes(index_id, label);
CREATE INDEX idx_nodes_ftype   ON nodes(index_id, file_type);
CREATE INDEX idx_nodes_file    ON nodes(index_id, source_file);
CREATE INDEX idx_edges_source  ON edges(index_id, source);
CREATE INDEX idx_edges_target  ON edges(index_id, target);
CREATE INDEX idx_edges_relation ON edges(index_id, relation);
```

## Cross-repo model: per-repo index + query-time join

Each manifest repo ingests its **own** `graph.json` into its **own** `index_id`. There are
no materialized cross-repo edges. Cross-repo relationships (who consumes this RPC/library —
needed by downstream layers) are computed by **joining across indexes at query time** by
matching identifiers (package, symbol label, module path). `store/` exposes a cross-index
query helper that downstream layers build `consumed_by` on.

This makes repo→index a clean 1:1, lets a single repo be re-indexed independently, and
removes any dependency on Graphify's `merge` step. Graphify's merged `graph.json` is **not**
the primary input (see Spec refinement).

## Ingestion (`index` command + loader)

1. Resolve manifest entry → upsert `repos` row → create/find `indexes` row for `(repo_id, commit_sha)`.
2. Parse `graph.json` (streamed to bound memory on large graphs).
3. In one transaction: delete existing `nodes`/`edges`/`hyperedges` for that `index_id`, then insert. **Idempotent** — re-indexing the same commit replaces cleanly.
4. Degradation: missing file → warn + skip; empty graph → empty index created; malformed node/edge → skip with warning, continue.

## MCP tools (I/O contracts — downstream extends these)

| Tool | Input | Output |
|------|-------|--------|
| `list_repos` | `{}` | `[{repo, branch, commit, indexed_at, node_count}]` |
| `resolve_repo` | `{query}` | best repo match (fuzzy org/name) + candidates |
| `query_repo` | `{repo, kind?, name?, relation?, file?, limit?}` | matching `nodes[]` and/or `edges[]` (deterministic, structural) |
| `explain_symbol` | `{repo, symbol \| node_id}` | `{node, source_file, source_location, callers[], callees[], edges[]}` |
| `get_file_context` | `{repo, file_path}` | symbols defined in file + their edges |

`query_repo` is structural SQLite only — no NL/semantic layer and no runtime Graphify
dependency. The MCP client (itself an LLM) composes structured queries.

## Resources

- `repo://org/name` → snapshot summary (branch, commit, counts).
- `graph://org/name/commit/<sha>/node/<node_id>` and `.../edges/<node_id>` → node/edge JSON.

Commit comes from the manifest; if absent, degrade to `branch` then `latest`.

## Error handling

Missing/empty graph, unknown repo, and unresolved symbol all return empty arrays or a
structured "not found" (with `resolve_repo`-style suggestions where useful) — never a crash.

## Testing strategy

- **Unit**: loader (fixture `graph.json` → row assertions), store queries (table-driven), registry resolution and fuzzy matching.
- **Golden**: each tool's JSON output against a committed fixture repo.
- **Degradation**: empty graph, missing file, unknown repo, unresolved symbol → empty/structured, never error.
- **E2E**: launch `serve` over stdio, run MCP `initialize` + `tools/list` + one representative tool call, assert protocol responses.
- Pure-Go SQLite enables fast temp-file / in-memory DBs in tests.

## Spec refinement (written back to delta spec)

Open-phase `tasks.md` treated "cross-repo merged graph input" as a primary ingestion path.
This design makes **per-repo indexing primary** and cross-repo a **query-time join**;
merged-graph input is deferred/optional. Reflected in the `graph-index` acceptance scenarios
and to be adjusted in `tasks.md` (task 3.3) during build.

## Out of scope (deferred)

NL/semantic query, materialized cross-repo edges, merged-graph ingestion as primary,
any contract/dependency parsing (those are changes 2–4), breaking-change detection.
