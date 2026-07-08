## Context

candle exposes 13 MCP tools today, each a thin pure method on `*Tools` over the
SQLite store, registered in `internal/mcp/server.go`. Resolution goes through
`t.reg.Resolve(repo) -> (registry.RepoInfo, ok, err)`; `RepoInfo` carries `IndexID`,
`Repo` (`org/name`), `Branch`, and `Commit`. The precise tools already implement every
underlying query `get_context` needs to compose (`NodesByLabel`, `Callers`/`Callees`,
`ListAPISpecs`/`FindOperations`, `ListProtoFiles`/`FindRPCs`, `FindPrivateLibraries`/
`FindPrivateDeps`, plus `Tools.FindSchema`, `Tools.ExplainRPC`, `Tools.FindPrivateLibrary`).

This change is grounded in the existing plan at
`docs/superpowers/plans/2026-06-18-get-context-retrieval-facade.md`, validated against the
current codebase.

## Goals / Non-Goals

**Goals**
- One additive `Tools.GetContext` method exposing overview and topic retrieval.
- Reuse existing store/Tools queries — no new store methods, no parser changes.
- Codegraph-style one-hop caller/callee context for matched code symbols.
- Explicit, machine-readable `limitations` so agents know what is deferred.

**Non-Goals (deferred; surfaced as runtime `limitations` strings)**
- OpenAPI endpoint → handler implementation linking inside `get_context` v1.
- Cross-repo RPC `consumed_by` aggregation.
- Cross-repo private-library consumer aggregation.
- Embeddings / semantic search and multi-hop traversal (`depth > 1`; v1 supports one hop).

## Decisions

### D1: Pure method + thin registration, mirroring existing tools
`Tools.GetContext(GetContextArgs) (ContextResult, error)` holds all logic; `server.go` adds
a `registerGetContext` that marshals the result via the existing `textResult`/`mustJSON`/
`toolErr` helpers. Keeps `get_context` consistent with the other 13 tools and SDK-free in
the method itself.

### D2: Single tool, two modes, mode filter
`topic` empty → overview (capability catalog). `topic` set → topic retrieval. A `mode`
argument (`overview|code|api|proto|library|all`, default/unknown → `all`) narrows the
searched surfaces. This avoids proliferating tools while serving both Context7-style
discovery and focused lookup.

### D3: Typed repo summary, not `any`
The result's repo field SHALL be a typed value (embed/return `registry.RepoInfo` or a
dedicated typed repo-summary struct) so callers and tests can access `.Repo` and `.Commit`
directly. **This resolves the inconsistency in the source plan**, whose draft declared
`Repo any` yet whose test accesses `out.Repo.Repo` / `out.Repo.Commit` — that would not
compile. The build phase MUST use a typed field. The exact struct shape is a build-phase
detail; the constraint is: typed, with accessible `Repo` and `Commit`.

### D4: One-hop code context via existing edge queries
Code matches reuse `NodesByLabel` + `Callers`/`Callees` (one hop), matching the shape
`explain_symbol` already returns. `depth` is accepted but v1 honors only one hop; deeper
traversal is a non-goal recorded in `limitations`.

### D5: Resource URIs reuse existing schemes
`include_resources` emits URIs in the established `graph://`, `openapi://`, `proto://`,
`lib://` schemes (commit-pinned, falling back to `latest` when commit is empty), so hints
are directly usable against the resource layer.

## Risks / Trade-offs

- **Surface-count drift:** advertised tool count moves 13 → 14; `e2e_surface_test.go`
  comments and assertions (≈ lines 32, 218) must be updated or the suite fails. Mitigated
  by an explicit task.
- **Overlap with precise tools:** `get_context` intentionally duplicates entry points the
  precise tools expose. Accepted — it is a router, and responses point back to the precise
  tools via `suggested_next_calls`.
- **Result-type breadth:** several result sub-fields stay loosely typed (`any`) where they
  wrap existing heterogeneous results; D3 constrains only the repo field, which the tests
  exercise directly.

## Migration Plan

Additive only. No data migration, no behavior change to existing tools. Ship the method +
registration + test-surface bump together so `tools/list` and the e2e surface stay
consistent within a single change.

## Open Questions

- Final concrete shape of the typed repo-summary struct (D3) — resolved during build/TDD.
- Whether `tdd_mode` is `tdd` (natural fit; plan is already test-first) — confirmed at the
  build-phase decision point, not here.
