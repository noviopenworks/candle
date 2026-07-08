## Why

candle can locate an internal library (`find_private_library`) and report a single
repo's usage of it (`find_library_consumers`), but the latter explicitly **defers
cross-repo consumer aggregation** — it returns a `deferred` marker for the cross-repo
dimension. So no tool answers "who across the whole org uses this internal library, at
which versions, and which of its symbols?" That cross-repo, both-sides view is the whole
point of indexing a private library from both provider and consumer sides, and the
`get_context` facade lists the same gap as a known limitation.

## What Changes

- Add a new MCP tool `explain_private_library` that explains an internal Go library from
  **both sides in one call**:
  - **Provider side:** resolve the library globally, return its packages, exports
    (package / symbol / kind / doc), doc synopsis, and the defining repo/commit; link each
    export to its **provider code-graph node**.
  - **Consumer side:** aggregate across **all** indexed repos — for each consuming repo:
    pinned version, used packages, and used symbols with file:line; **best-effort** link
    each usage to a **consumer code-graph node**.
  - **Input:** a fuzzy `query` (name / module path / synopsis / readme) resolving to a
    best-match library, returning candidates when ambiguous (mirrors `resolve_repo`).
  - Every response carries explicit `limitations` for deferred behavior and an explicit
    marker for any usage whose consumer code-graph node could not be resolved.
- Add a cross-index store query that aggregates private-library usages/dependencies by
  module path across all indexes, joined to repo identity.
- Register `explain_private_library` as the 15th advertised MCP tool.
- Update docs (`docs/tools.md`, `docs/examples.md`, `README.md`).

Non-breaking: `find_private_library` and `find_library_consumers` keep their current
behavior (the latter keeps its single-repo scope and deferred marker); `explain_private_library`
is the tool that delivers the cross-repo dimension.

## Capabilities

### New Capabilities
<!-- None; this extends the existing private-library-tools capability. -->

### Modified Capabilities
- `private-library-tools`: ADD a requirement for `explain_private_library` — a both-sides,
  cross-repo library explanation with code-graph linking. Existing requirements for
  `find_private_library` and `find_library_consumers` are unchanged.

## Impact

- **New code:** cross-index aggregation query in `internal/store/godep.go`;
  `Tools.ExplainPrivateLibrary` + result types in `internal/mcp` (likely
  `internal/mcp/godep_tools.go` or a new `internal/mcp/library_explain.go`); MCP
  registration in `internal/mcp/server.go` (`ToolNames` + register fn); surface-count
  update in `internal/mcp/e2e_surface_test.go`.
- **Reused:** `PrivateLibraryByModule` (provider), `FindPrivateLibraries`/`FindPrivateDeps`
  (fuzzy resolution), `NodesByLabel` (graph linking), registry repo identity.
- **Docs:** `docs/tools.md`, `docs/examples.md`, `README.md` (tool count 14 → 15).
- **No change** to parsers, ingestion, or existing tool outputs.
- **Dependencies:** none added.
