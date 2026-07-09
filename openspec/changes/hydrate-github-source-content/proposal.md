## Why

candle can link Graphify nodes back to source metadata, but agents often need the surrounding source text to disambiguate same-named symbols or understand a file without re-indexing repository contents. On-demand source hydration lets MCP callers retrieve GitHub-backed file content only when it is useful, keeping default graph responses lightweight.

## What Changes

- Add opt-in source-content hydration for Graphify-backed MCP results.
- Use externally authorized or already-fetchable GitHub raw/source URLs from indexed Graphify metadata; candle will not introduce its own GitHub credential store in this change.
- Automatically hydrate source content when the caller opts in and a result is ambiguous, source location is missing, or a source resource is explicitly requested.
- Support both line-window snippets around a source location and capped whole-file text responses.
- Return fetch status and metadata when content is unreachable, unsupported, oversized, or non-text instead of failing the entire MCP tool call.

## Capabilities

### New Capabilities

- `source-content-hydration`: On-demand retrieval of GitHub source file content for Graphify-backed MCP tools and resources.

### Modified Capabilities

- None.

## Impact

- MCP tool inputs and outputs for `get_context`, `query_repo`, `explain_symbol`, and `get_file_context` gain explicit source hydration behavior.
- Graph node query rows need to expose enough provenance, especially `source_url`, to resolve fetchable GitHub content.
- The MCP server will perform bounded network fetches for text source content when requested, with limits and clear failure statuses.
- Documentation and tests must cover opt-in behavior, ambiguity-triggered hydration, snippet/full-file modes, and unreachable or oversized content handling.
