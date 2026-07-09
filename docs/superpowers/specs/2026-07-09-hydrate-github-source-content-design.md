---
comet_change: hydrate-github-source-content
role: technical-design
canonical_spec: openspec
---

# Hydrate GitHub Source Content Design

## Context

candle already stores Graphify node provenance in SQLite, including `source_file`, `source_location`, `source_url`, `captured_at`, `author`, and `contributor`. The current query-facing `store.NodeRow` exposes only file and location metadata, so MCP tools can identify symbols and files but cannot retrieve source text from GitHub when metadata alone is insufficient.

The affected MCP paths are `get_context`, `query_repo`, `explain_symbol`, and `get_file_context`. They currently return metadata-only JSON through pure tool methods and SDK wrappers in `internal/mcp/server.go`. Resource handlers also return metadata-only JSON. The new behavior must preserve those defaults and make source fetching explicit.

## Goals

- Preserve metadata-only defaults for existing clients.
- Add a structured `source_content` option to the affected Graphify-backed tools.
- Add a dedicated direct source-read MCP tool for explicit source reads.
- Centralize GitHub URL normalization, fetching, text detection, limits, snippets, and fetch statuses.
- Return structured per-source status instead of failing whole tool calls when content cannot be fetched.

## Non-Goals

- No fetched file content is persisted in SQLite.
- No candle-managed GitHub auth or token configuration is added.
- No non-GitHub provider abstraction is added in this change.
- No binary or unbounded file streaming is supported.
- No Graphify extraction changes are required.

## Architecture

Use a shared hydration layer inside `internal/mcp` and keep tool-specific code focused on selecting nodes/files and shaping responses.

```
MCP tool args
    |
    v
pure tool method selects nodes/files
    |
    v
source_content option present?
    | no                         | yes
    v                            v
existing metadata result     shared source hydrator
                                 |
                                 v
                         SourceContent envelope
                                 |
                                 v
                         wrapper MCP response
```

### Store Provenance

Extend `store.NodeRow` and `nodeCols` in `internal/store/query.go` to include the provenance columns already present in `nodes`: `source_url`, `captured_at`, `author`, and `contributor`. This is not a schema migration because `internal/store/schema.go` already defines those columns and `internal/graph/loader.go` already inserts them.

This change lets all existing node query helpers return enough provenance for source hydration and for metadata-only responses to expose the stored source URL.

### Source Hydration Types

Add small SDK-free types in `internal/mcp`, likely in a new `source_content.go` file:

```go
type SourceContentOptions struct {
    Mode          string `json:"mode,omitempty"`          // off|auto|snippet|full
    MaxBytes      int    `json:"max_bytes,omitempty"`
    LineRadius    int    `json:"line_radius,omitempty"`
    MaxCandidates int    `json:"max_candidates,omitempty"`
}

type SourceContent struct {
    Status      string `json:"status"` // fetched|skipped|unsupported|error
    Mode        string `json:"mode,omitempty"`
    SourceFile  string `json:"source_file,omitempty"`
    SourceURL   string `json:"source_url,omitempty"`
    StartLine   int    `json:"start_line,omitempty"`
    EndLine     int    `json:"end_line,omitempty"`
    Truncated   bool   `json:"truncated,omitempty"`
    Content     string `json:"content,omitempty"`
    Reason      string `json:"reason,omitempty"`
}
```

Use constants for defaults in v1 rather than adding config. Proposed defaults: 64 KiB maximum fetched content, 20 lines of radius for snippets, 5 hydrated candidates for ambiguous results, and a 5 second HTTP timeout. The implementation can adjust exact constants if tests show they are too high or low.

### Hydrator

Add a shared hydrator with responsibilities split into small functions:

- Normalize GitHub source URLs.
- Convert `https://github.com/<org>/<repo>/blob/<ref>/<path>` to `https://raw.githubusercontent.com/<org>/<repo>/<ref>/<path>`.
- Accept existing `raw.githubusercontent.com` URLs directly.
- Reject unsupported hosts with `status: unsupported`.
- Construct a raw GitHub URL from repo identity, commit or branch, and `source_file` when no `source_url` is present and enough snapshot data exists.
- Fetch with a bounded `http.Client` timeout and byte limit.
- Detect non-text content using content type and a small byte scan.
- Extract snippet windows from `source_location` when mode is `snippet` or automatic snippet hydration selects a node with a location.
- Return structured `SourceContent` statuses for every outcome.

Network errors, unsupported URLs, non-text content, missing provenance, and oversized content are data statuses. They should not become tool errors unless the underlying repo or symbol lookup itself fails.

### Tool Arguments

Each affected tool gets an optional structured field:

```go
SourceContent *SourceContentOptions `json:"source_content,omitempty"`
```

`nil` or `mode: off` means existing metadata-only behavior. `mode: auto` applies trigger rules. `mode: snippet` and `mode: full` explicitly request snippets or capped full-file content.

Automatic hydration triggers only when the caller opted in and at least one trigger is true:

- `query_repo` returns multiple matching nodes.
- A selected node lacks `source_location` and has fetchable source provenance.
- `get_file_context` is called with source content enabled for a file.
- `get_context` includes code-symbol matches whose ambiguity or missing location makes metadata insufficient.

### Tool Responses

Keep old JSON shapes when `source_content` is omitted. When hydration is enabled, return wrapper results instead of mutating base rows.

- `query_repo`: default remains `[]store.NodeRow`; hydrated response becomes `[]SourceNodeResult` with `{node, source_content}`.
- `explain_symbol`: default remains `SymbolExplanation`; hydrated response becomes `{explanation, source_content}` for the resolved node.
- `get_file_context`: default remains `[]store.NodeRow`; hydrated response becomes `{file, symbols, source_content}`.
- `get_context`: default remains `ContextResult`; hydrated response keeps the context envelope but uses hydrated code-symbol wrappers where source content is requested.

The server registration layer may return `any` from pure methods or branch before JSON marshalling so default calls keep their current serialized shape.

### Direct Source-Read Tool

Add a new MCP tool named `read_source_content`.

Suggested arguments:

```go
type readSourceContentArgs struct {
    Repo          string                `json:"repo"`
    NodeID        string                `json:"node_id,omitempty"`
    File          string                `json:"file,omitempty"`
    SourceContent *SourceContentOptions `json:"source_content,omitempty"`
}
```

The caller must provide either `node_id` or `file`. If `node_id` is provided, resolve the node and hydrate from its provenance. If `file` is provided, resolve the file through `NodesByFile`; use the first node with fetchable provenance for the file, and include a clear `skipped` or `unsupported` status if no node can resolve content.

The new tool returns the same `SourceContent` envelope used by hydrated wrapper results. It does not introduce a new resource URI in this change.

### Data Flow

```
Graphify graph.json
    |
    v
nodes.source_url stored by loader
    |
    v
NodeRow exposes source_url
    |
    v
tool result selects node/file
    |
    v
hydrate(node, repo snapshot, options)
    |
    +--> normalize GitHub URL
    +--> fetch bounded text
    +--> window or cap content
    +--> return SourceContent status
```

## Error Handling

- Repo, node, and file lookup failures remain normal tool errors or not-found errors.
- Source fetch failures become `SourceContent{Status: "error", Reason: ...}`.
- Unsupported hosts or missing URLs become `Status: "unsupported"` or `Status: "skipped"`.
- Non-text content omits `content` and reports a reason.
- Truncated content sets `truncated: true` and includes the capped body.
- Hydration limits should be deterministic so tests do not depend on external GitHub behavior.

## Testing Strategy

Add unit tests for the shared hydrator before wiring all tools:

- Raw GitHub URL pass-through.
- GitHub blob-to-raw conversion.
- Unsupported host rejection.
- Missing provenance status.
- Bounded fetch and truncation.
- Non-text content detection.
- Source-location snippet extraction.

Use `httptest.Server` where possible so tests avoid external network calls. Add MCP tool tests for metadata-only defaults and opt-in hydrated wrapper responses for `query_repo`, `explain_symbol`, `get_file_context`, and `get_context`. Add direct tool tests for `read_source_content` by node and by file. Update e2e surface tests and docs because the advertised MCP tool surface changes.

## Spec Patches Applied

The OpenSpec delta spec and task list were patched to include the dedicated direct source-read MCP tool. No broader requirement rewrite was needed.

## Implementation Decisions

- Use these v1 defaults unless implementation tests reveal a correctness issue: 64 KiB maximum content, 20-line snippet radius, 5 hydrated candidates, and 5 second HTTP timeout.
- For existing enrichment tools, omitted `source_content` means off; a present empty `source_content` object means automatic mode.
- For `read_source_content`, omitted `source_content` means snippet mode for node reads with a source location and full capped mode for file reads or node reads without a source location.
- `read_source_content` returns one `SourceContent` envelope. `get_file_context` remains the tool that combines file symbols with optional source content.
