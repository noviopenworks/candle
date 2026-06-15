# Brainstorm Summary

- Change: mcp-core-foundation
- Date: 2026-06-15

## Confirmed Technical Approach

- **Stack**: Go single binary. `modernc.org/sqlite` (pure Go, no cgo) for storage; official MCP Go SDK (`modelcontextprotocol/go-sdk`) for the stdio server; **cobra** for the CLI; **viper** for config.
- **Repo registry**: explicit **viper manifest** — each entry = `org/repo`, graph.json path, commit SHA, branch. Source of the `repo` arg and enables commit-pinned resources.
- **Cross-repo model**: **per-repo index + query-time join**. Each manifest repo ingests its own `graph.json` into its own `index_id`. Cross-repo relations computed by joining across indexes at query time. No materialized cross-repo edges; merged-graph input is NOT the primary path.
- **Query path**: structural, **SQLite-native** (no NL/semantic engine, no runtime Graphify dependency). The MCP client (an LLM) forms structured queries.
- **`index_id`** = one indexed repo snapshot (repo_id + commit). Every downstream contract/dependency table references it.
- **Repo layout**: `cmd/` (cobra: `serve`, `index`); `internal/` (`config`, `registry`, `store`, `graph`, `mcp/{tools,resources}`).

## Key Trade-offs and Risks

- Pure-Go sqlite: slightly slower than C, fine for read-heavy graph queries; keeps single static binary + easy cross-compile.
- Official Go SDK is young; if API is unstable, fall back to `mark3labs/mcp-go` behind the same internal interface.
- Per-repo + query-time join: cross-repo accuracy depends on identifier matching across indexes (the real work lands in downstream layers); foundation provides the cross-index query helper.
- Hard dependency on Graphify `graph.json` schema (nodes/edges/hyperedges) — schema drift ripples here.

## Testing Strategy

- Unit: graph loader (fixture graph.json → row assertions), store queries (table-driven), registry resolution.
- Golden tests for each tool's JSON output against a fixture repo.
- Degradation tests: empty graph, missing file, unknown repo/symbol → empty results, never errors.
- E2E: launch server over stdio, run MCP `initialize` + `tools/list` + one tool call, assert protocol responses.

## Spec Patches

- Refinement (not a rewrite): open-phase tasks.md treated "cross-repo merged graph input" as a primary ingestion path; design decision makes per-repo indexing primary and cross-repo a query-time join. To reflect in tasks.md during build (task 3.3) and in the `graph-index` delta spec acceptance scenarios.
- New delta specs to create for declared capabilities `graph-index` and `mcp-core` (with acceptance scenarios), since open phase created only proposal/design/tasks.
