# Design — add-openapi-handler-linking (roadmap 0.2)

> High-level architecture decisions for the open phase. The deep technical design
> (handler-identification signal, scoring tiers, exact AST shape) is finalized in
> the Comet design phase.

## Context

`explain_rpc` returns `implemented_by` because the ingest flow runs `MatchRPCs`
(`internal/link/link.go:36`), which writes `store.RPCImplLink` rows via
`LinkRPCImpls`, and `proto_tools.go` reads them back. The HTTP path has no
equivalent. The `link` package was deliberately built generic for this:

```
internal/link/link.go
  MatchRPCs ──┐
              ├─ readSourceUnderRoot(root, sourceFile)   (shared)
  MatchExports┤   astSignatureMatch / scoring tiers      (shared, reusable)
              └─ score() → confHigh/confMedium/confLow
```

## Approach

Mirror the proven RPC linking pipeline for HTTP operations end to end:

```
ingest.Run
  ├─ MatchRPCs(...)        → LinkRPCImpls       (exists)
  └─ MatchOpenAPI(...)     → LinkHTTPOpImpls    (new, this change)

store: new http_operation_impls table (keyed by index_id + operation identity)
mcp:   ExplainEndpoint reads links → response.implemented_by[]
```

### Decisions (locked at open)

1. **Reuse, don't fork, the AST scaffolding.** `MatchOpenAPI` lives in
   `internal/link` and reuses `readSourceUnderRoot` and the confidence constants.
   AST is authoritative; absence of `root` falls back to the name tier (no false
   HIGH), exactly as RPC linking does.
2. **Separate store linkage, parallel to RPCs.** Add an HTTP-op impl-link table and
   `LinkHTTPOpImpls` / reader, rather than overloading `RPCImplLink`. Keeps the two
   contract kinds independent and the schema readable.
3. **Additive MCP response.** `explain_endpoint` gains `implemented_by[]`; no
   existing field changes. Empty list when nothing is linked.
4. **e2e parity.** Extend the existing e2e to assert HTTP `implemented_by` the same
   way the proto path is asserted today.

### Central open question (resolved in design phase)

**How is an operation tied to a handler candidate?** Options, to be decided with
the fresh look at real Go HTTP handler shapes:
- **operationId ↔ handler function name** (e.g. `reserveProduct` ↔ `ReserveProduct`)
  — simplest, matches how the RPC linker keys off the RPC name.
- **route-registration signal** — find the router call that binds `method`+`path`
  to a handler (analog of the RPC "service registration" signal that earns MEDIUM).
- **AST confirmation** — candidate must have an HTTP-handler signature
  (`http.ResponseWriter`, `*http.Request`) to earn HIGH.

Likely a layered combination (name → registration → AST), mirroring
`score()`'s `name` → `name+service` → `name+service+ast` ladder. The handler
signal varies by router library; the design phase will pick the MVP signal set
and document which routers are covered.

## Risks / unknowns

- Router diversity (net/http, chi, gin, echo) makes the registration signal
  library-specific; MVP may cover the AST signature + operationId-name path and
  treat router-specific binding as a later enhancement.
- operationId is optional in OpenAPI; need a method+path fallback identity.

## Out of scope

gRPC linking (exists), `consumed_by` aggregation (1.4), multi-hop traversal (1.3),
breaking-change detection, the demo itself (change `add-runnable-demo`).
