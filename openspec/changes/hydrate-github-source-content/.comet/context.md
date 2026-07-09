# Comet Design Handoff

- Change: hydrate-github-source-content
- Phase: design
- Mode: compact
- Context hash: 569e599ba2c3d02011fd695b6cb3c8b2d5fc65e5397ad743e18e4e41e41cccf8

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/hydrate-github-source-content/proposal.md

- Source: openspec/changes/hydrate-github-source-content/proposal.md
- Lines: 1-28
- SHA256: b58b8793babf1ad249b3c52583ddd99bf7a9a55d8846cc0ac7f5a940ae25a8b6

```md
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

```

## openspec/changes/hydrate-github-source-content/design.md

- Source: openspec/changes/hydrate-github-source-content/design.md
- Lines: 1-69
- SHA256: 9dfb864bf41e011b65db499827d9617a7faef2c8658d51b63a422cde2a4ff3ed

```md
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

```

## openspec/changes/hydrate-github-source-content/tasks.md

- Source: openspec/changes/hydrate-github-source-content/tasks.md
- Lines: 1-23
- SHA256: 5de796496110594f94e1b2b48d3c7ed00320c70d21e3146a3667619641ff2116

```md
## 1. Provenance and Hydration Core

- [ ] 1.1 Expose stored node provenance (`source_url`, `captured_at`, `author`, `contributor`) through query-facing store rows without changing default metadata-only behavior.
- [ ] 1.2 Add a source hydration request/response model with modes for `off`, automatic snippets, explicit snippets, and capped full-file content.
- [ ] 1.3 Implement GitHub source URL normalization for raw URLs and convertible blob URLs, returning a structured unsupported status for other URLs.
- [ ] 1.4 Implement bounded text fetching with timeout, byte cap, text detection, line-window extraction, truncation reporting, and per-source error status.

## 2. MCP Tool Integration

- [ ] 2.1 Add source hydration options to `query_repo`, preserving existing output when the option is absent.
- [ ] 2.2 Add source hydration options to `explain_symbol`, preserving existing output when the option is absent.
- [ ] 2.3 Add source hydration options to `get_file_context`, including explicit file/resource-triggered hydration.
- [ ] 2.4 Add source hydration options to `get_context`, including automatic hydration for ambiguous matches or missing source locations when enabled.
- [ ] 2.5 Add a dedicated direct source-read MCP tool that accepts repo plus node or file reference and returns the structured source-content envelope.
- [ ] 2.6 Register the new source-read tool in the MCP server and update the e2e advertised tool surface expectations.

## 3. Tests and Documentation

- [ ] 3.1 Add unit tests for URL normalization, unsupported URL handling, fetch limits, text detection, snippet extraction, and truncation status.
- [ ] 3.2 Add MCP tool tests proving default metadata-only behavior and opt-in hydration behavior for `query_repo`, `explain_symbol`, `get_file_context`, and `get_context`, plus direct source reads through the new tool.
- [ ] 3.3 Add failure-path tests for unreachable GitHub content, non-text content, missing `source_url`, and oversized responses.
- [ ] 3.4 Update docs for the new MCP source hydration options, response envelope, limits, and Graphify `source_url` expectations.
- [ ] 3.5 Run `go test ./...` and address any regressions before entering verification.

```

## openspec/changes/hydrate-github-source-content/specs/source-content-hydration/spec.md

- Source: openspec/changes/hydrate-github-source-content/specs/source-content-hydration/spec.md
- Lines: 1-63
- SHA256: 478abfac8c75e4155c681599b18f762c077701b3e376a66776bd1514e5368017

```md
## ADDED Requirements

### Requirement: Source hydration is opt-in for Graphify-backed MCP tools
The system SHALL keep `get_context`, `query_repo`, `explain_symbol`, and `get_file_context` metadata-only by default, and SHALL expose an explicit source hydration option that allows callers to request GitHub source content for Graphify-backed results.

#### Scenario: Default tool calls remain metadata-only
- **WHEN** a caller invokes a Graphify-backed MCP tool without the source hydration option
- **THEN** the response SHALL NOT fetch or include source file content

#### Scenario: Caller enables automatic source hydration
- **WHEN** a caller invokes a Graphify-backed MCP tool with source hydration mode set to automatic
- **THEN** the system SHALL consider source content fetching only for ambiguous results, results missing source locations, or explicit source resource/file-context requests

### Requirement: GitHub source content is resolved from indexed provenance
The system SHALL resolve source content from indexed Graphify provenance, preferring a node `source_url` that points to GitHub raw content or a GitHub blob URL convertible to raw content.

#### Scenario: GitHub source_url is fetchable
- **WHEN** a hydrated result has a GitHub raw URL or convertible GitHub blob URL in `source_url`
- **THEN** the system SHALL fetch source content from the corresponding raw GitHub URL

#### Scenario: Source URL is unsupported or unavailable
- **WHEN** a hydrated result has no supported GitHub source URL
- **THEN** the system SHALL return the graph metadata with a source-content status explaining that content was not fetched

### Requirement: Hydrated content supports snippet and full-file modes
The system SHALL support a snippet mode that returns a bounded line window and a full-file mode that returns bounded text content for a source file.

#### Scenario: Snippet mode has a source location
- **WHEN** snippet hydration is requested for a node with a source location
- **THEN** the system SHALL return a bounded line window around that source location and include the returned line range

#### Scenario: Full-file mode exceeds the content cap
- **WHEN** full-file hydration is requested and the fetched text exceeds the configured content cap
- **THEN** the system SHALL return capped content and mark the source-content result as truncated

### Requirement: Source fetch failures do not fail the whole MCP response
The system SHALL return per-source fetch status for unavailable, unreachable, oversized, non-text, or unsupported content without failing the whole MCP tool call when graph metadata is still available.

#### Scenario: GitHub fetch returns an error
- **WHEN** source hydration is requested and the GitHub fetch fails
- **THEN** the MCP tool response SHALL still include the relevant graph result and SHALL include a source-content status with the failure reason

#### Scenario: Source content is non-text
- **WHEN** source hydration is requested and the fetched content is detected as non-text
- **THEN** the MCP tool response SHALL omit the content body and SHALL include a source-content status indicating unsupported content type

### Requirement: Hydration responses are bounded and structured
The system SHALL represent hydrated source content in a structured envelope that includes fetch status, mode, source URL, source file, optional line range, truncation status, and content only when content is successfully fetched.

#### Scenario: Agent receives hydrated ambiguous matches
- **WHEN** `query_repo` returns multiple matching nodes and automatic source hydration is enabled
- **THEN** each hydrated candidate SHALL include a source-content envelope that helps distinguish the candidates without changing the node identity fields

### Requirement: Direct source reads are available through a dedicated MCP tool
The system SHALL expose a dedicated MCP tool for explicit source-content reads, while existing Graphify-backed tools continue to support source enrichment through their structured source hydration option.

#### Scenario: Caller reads source content directly
- **WHEN** a caller invokes the dedicated source-read MCP tool with a repo and node or file reference
- **THEN** the system SHALL return the same structured source-content envelope used by hydrated tool results

#### Scenario: Existing tools enrich results and direct tool reads exact content
- **WHEN** a caller needs automatic disambiguation in a lookup tool and later needs a direct source read
- **THEN** the lookup tool SHALL be able to return wrapper results with source-content status and the dedicated tool SHALL be able to return explicit source content for the selected node or file

```
