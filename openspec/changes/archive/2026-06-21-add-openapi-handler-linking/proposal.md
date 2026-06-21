## Why

candlegraph's flagship promise is *"where is this contract implemented?"*, yet
`explain_endpoint` answers it only for the contract side: it returns the OpenAPI
operation with **no link to the handler that implements it**. The gRPC path already
does this (`explain_rpc` returns `implemented_by` via the AST linker); the HTTP path —
the majority case for most services — does not. The gap is admitted in code at
`internal/mcp/openapi_tools.go:52` ("contract data only — no handler/service_flow")
and `internal/mcp/context_tools.go:286` ("OpenAPI endpoint implementation linking is
not yet available"). This is roadmap item **0.2**, and it unblocks the runnable-demo
exit criterion (item 0.1), whose flagship question is literally *"which handler
implements endpoint X?"*.

## What Changes

- Add `MatchOpenAPI` to `internal/link` — an HTTP-operation→handler matcher that
  reuses the package's existing generic AST scaffolding (`readSourceUnderRoot`,
  confidence tiering). The package doc already states it is *"intentionally generic
  so the OpenAPI handler linker can adopt it later."*
- Add store persistence for HTTP-operation→handler links, analogous to
  `RPCImplLink` / `LinkRPCImpls`.
- Wire `MatchOpenAPI` into the ingest flow alongside `MatchRPCs`, gated on the
  manifest `root` for AST (same as RPC linking).
- `explain_endpoint` returns an `implemented_by[]` field with handler node(s) and a
  confidence tier. **BREAKING (additive)**: the response gains a field; existing
  fields are unchanged.
- Update the stale limitation strings in `context_tools.go`.
- Extend the e2e surface to assert HTTP `implemented_by`, mirroring the existing
  proto assertion.

## Capabilities

### New Capabilities
<!-- none; this extends existing capabilities -->

### Modified Capabilities
- `ast-linking`: the linker, currently RPC- and export-only, gains AST-confirmed
  **HTTP handler** matching (a new requirement); source resolution and graceful
  fallback requirements are reused unchanged.
- `openapi-tools`: the `explain_endpoint returns contract data` requirement is
  amended — it SHALL now also return `implemented_by` handler links, replacing the
  current "SHALL NOT return handler implementation (linking is deferred)" clause.

## Impact

- **Code**: `internal/link` (new `MatchOpenAPI`), `internal/store` (new HTTP-op
  impl-link table + writer/reader), `internal/ingest` (wire-in), `internal/mcp`
  (`openapi_tools.go` response, `context_tools.go` limitation strings), e2e tests.
- **MCP surface**: `explain_endpoint` response gains `implemented_by`; no new tool,
  no removed field — additive and backward compatible.
- **Data**: a new SQLite table hanging off `index_id`; no change to existing tables.
- **Dependencies**: none new (stdlib `go/ast` already used by the linker).
- **Downstream**: unblocks change `add-runnable-demo` (roadmap 0.1).
