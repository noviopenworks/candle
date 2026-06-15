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
