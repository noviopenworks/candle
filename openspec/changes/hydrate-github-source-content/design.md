## Context

candle consumes Graphify `graph.json` files and stores node provenance such as `source_file`, `source_location`, and `source_url`. MCP tools currently expose symbol and file metadata, but they do not retrieve source text, so agents must ask for more context through external mechanisms when a symbol lookup is ambiguous or a file-level explanation needs actual code.

The SQLite schema already stores `source_url`, but the query-facing `NodeRow` type omits it. Existing MCP tools (`get_context`, `query_repo`, `explain_symbol`, and `get_file_context`) therefore have enough graph structure to identify candidates, but not enough exposed provenance to hydrate GitHub source content.

## Goals / Non-Goals

**Goals:**

- Add an explicit MCP source-hydration option for Graphify-backed tool results.
- Fetch GitHub source text only when the caller opts in and a configured trigger applies.
- Support both source-location snippets and capped whole-file content.
- Preserve lightweight metadata-only responses as the default behavior.
- Return per-file fetch status so unavailable content does not fail the whole tool response.

**Non-Goals:**

- Do not persist fetched file bodies in SQLite.
- Do not add a candle-managed GitHub credential store or token configuration.
- Do not support binary files, unbounded large files, or non-GitHub source hosts in the first version.
- Do not change Graphify extraction or require `graph.json` to embed file contents.

## Decisions

1. **Expose stored provenance before fetching content.**
   Extend query-facing node results to include `source_url` and related provenance already present in the `nodes` table. This keeps the hydration feature grounded in indexed Graphify data and avoids adding a separate lookup path for basic metadata.

   Alternative considered: fetch from `source_file` only. That would work for local fixtures, but it would not satisfy the GitHub streaming use case and would encourage unsafe local path reads in MCP serving.

2. **Use an explicit source hydration option with an automatic trigger mode.**
   Existing tool behavior remains metadata-only unless the caller passes a source-content option. Within that opt-in mode, `auto` hydrates only when results are ambiguous, source locations are missing, or a source resource/file context is explicitly requested.

   Alternative considered: always hydrate on ambiguity. That would surprise existing MCP clients with network latency and larger responses.

3. **Fetch only GitHub text content from externally authorized URLs.**
   Prefer `source_url` when it is a GitHub raw URL or a GitHub blob URL that can be converted to a raw URL. When `source_url` is absent, construct a raw GitHub URL only from repo identity, commit or branch, and `source_file` when those fields are available. candle will not manage tokens; URLs must already be public or externally authorized/fetchable.

   Alternative considered: add first-class GitHub token configuration. The user explicitly chose externally authorized URLs, so credential management stays out of this change.

4. **Return structured source content envelopes.**
   Hydrated responses should include metadata such as mode, source URL, line range, truncation, status, and error reason, plus content only when fetched successfully. The same envelope can be reused by all affected MCP tools.

   Alternative considered: append raw text directly to existing node rows. That would be hard for agents to distinguish from metadata and difficult to cap consistently.

5. **Apply conservative fetch limits.**
   Use a timeout, text-content detection, a maximum byte cap, and line-window extraction. Snippet mode should prefer `source_location` when available; full mode should still be capped and marked as truncated when the cap is reached.

   Alternative considered: stream arbitrary response bodies. MCP tool responses are not a safe place for unbounded content, and unbounded fetches would make e2e behavior unpredictable.

## Risks / Trade-offs

- Network fetches add latency and external failure modes -> Use opt-in hydration, bounded timeouts, and per-file status objects.
- Private GitHub content may be unavailable without auth -> Treat unavailable content as a fetch status, not a tool failure.
- Ambiguous symbol results may hydrate too much content -> Apply max candidate and byte limits, and report skipped entries when limits prevent hydration.
- GitHub URL formats vary -> Centralize URL normalization and test raw/blob URL conversion.
- Exposing source text can increase response size -> Keep metadata-only as default and make full-file mode capped.

## Migration Plan

No database migration is required because the `nodes` table already stores `source_url` and provenance fields. Existing indexed data remains valid. The implementation can add fields to response structs and MCP argument structs while preserving default behavior for clients that do not request source hydration.

Rollback is to disable or remove the new MCP options; metadata-only graph queries remain unchanged.

## Open Questions

- Exact field names for the source hydration option and content envelope should be finalized during Superpowers design.
- Default byte cap, line radius, timeout, and maximum hydrated candidates need concrete values.
- Resource URI shape for explicit source-content reads should be chosen to fit the existing MCP resource parser.
