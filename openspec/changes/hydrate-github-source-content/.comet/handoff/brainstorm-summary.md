# Brainstorm Summary

- Change: hydrate-github-source-content
- Date: 2026-07-09

## Confirmed Technical Approach

Adopt Approach A: implement a shared source hydrator, extend store node rows with existing provenance, expose structured `source_content` options on the affected tools, return wrapper hydrated results when source content is enabled, and add a direct source-read MCP tool for explicit file/node content retrieval.

Implementation boundaries:
- Keep existing tool responses metadata-only when `source_content` is omitted.
- Use a shared hydrator for GitHub URL normalization, fetch limits, text detection, snippet extraction, full-file caps, and per-source status.
- Use wrapper result types for hydrated responses, such as `{node, source_content}`, instead of mutating base node rows or returning a parallel map.
- Keep direct explicit source reads in a new MCP tool; existing tools still support automatic/snippet enrichment through `source_content`.

## Key Trade-offs and Risks

- Confirmed: prefer a structured hydration argument over flat fields, a minimal enum, or tool-specific option shapes.
- Confirmed: prefer wrapper result objects for hydrated responses.
- Confirmed: use both paths: existing-tool enrichment plus a new direct source-read MCP tool.
- Confirmed: shared hydrator architecture selected over direct-tool-only and per-tool inline hydration.
- Risk: GitHub fetch latency and network failures can slow tool calls. Mitigation: hydration is opt-in, bounded, and reports per-source status.
- Risk: private repository content may not be fetchable without auth. Mitigation: candle does not manage auth; URLs must already be externally authorized/fetchable, and failures are status values.
- Risk: large or binary files can bloat responses. Mitigation: text detection, byte caps, line windows, truncation flags, and skipped statuses.
- Candidate risk: network fetch latency and unavailable private content must be represented as per-source status, not whole-tool failure.
- Candidate risk: output size must be bounded by candidate, byte, and line-window limits.

## Testing Strategy

Unit-test the hydrator and URL normalization independently. Use test HTTP servers for fetch success, truncation, non-text content, unreachable content, and unsupported URL statuses. Add MCP tool tests proving metadata-only defaults plus opt-in wrapper results for `get_context`, `query_repo`, `explain_symbol`, and `get_file_context`. Update e2e surface tests for the new direct source-read tool.

## Spec Patches

Update the delta spec and task list to explicitly include the dedicated direct source-read MCP tool alongside existing-tool source enrichment.
