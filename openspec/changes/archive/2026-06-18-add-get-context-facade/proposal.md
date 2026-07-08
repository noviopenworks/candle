## Why

Agents querying candle today must already know which precise tool to reach for
(`explain_symbol`, `find_endpoint`, `find_rpc`, `find_private_library`, …) and how a
repo's knowledge is organized before they can ask anything useful. There is no single
"start here" entry point that tells an agent what candle knows about a repo and
routes it to the right follow-up. This mirrors the Context7 retrieval pattern: one call
to discover capabilities, then focused topic lookups.

## What Changes

- Add a new MCP tool `get_context` as the primary, Context7-style retrieval entry point.
  - **Overview mode** (`repo` only): returns a repo snapshot summary plus a catalog of
    available capabilities (code graph, OpenAPI, protobuf, private libraries) with counts,
    the precise tools that serve each surface, suggested next calls, and resource schemes.
  - **Topic mode** (`repo` + `topic`): searches all surfaces for the topic and returns
    code symbols with one-hop callers/callees (codegraph-style), matched HTTP operations,
    schemas/proto messages, RPCs, and private libraries, plus exact resource-URI hints.
  - `mode` filter (`overview|code|api|proto|library|all`) narrows which surfaces are searched.
  - Every response carries explicit `limitations` describing what is deferred.
- Register `get_context` as the 14th advertised MCP tool (after `resolve_repo`).
- Update user-facing docs (`docs/tools.md`, `docs/examples.md`, `README.md`) to make
  `get_context` the recommended first call, with precise tools as follow-ups.

Non-breaking: existing tools and their behavior are unchanged; `get_context` is additive
and composes existing store queries.

## Capabilities

### New Capabilities
- `context-retrieval`: a repo-scoped retrieval facade (`get_context`) that composes existing
  code-graph, OpenAPI, protobuf, and private-library queries into overview and topic-oriented
  context for agent consumption.

### Modified Capabilities
<!-- None. mcp-core's existing tool requirements are unchanged; the advertised tool count
     moves from 13 to 14 but that is captured by the new context-retrieval capability and a
     test-surface update, not a behavioral change to existing tools. -->

## Impact

- **New code:** `internal/mcp/context_tools.go` (the `Tools.GetContext` method + result types),
  `internal/mcp/context_tools_test.go`.
- **Modified code:** `internal/mcp/server.go` (`ToolNames` + registration),
  `internal/mcp/e2e_surface_test.go` (advertised tool count 13 → 14, comments at lines ~32/218).
- **Docs:** `docs/tools.md`, `docs/examples.md`, `README.md` (tool count + retrieval-first flow).
- **No change** to `internal/store`, parsers, the registry, or any existing tool's output.
- **Dependencies:** none added; reuses the MCP Go SDK registration pattern already in `server.go`.
