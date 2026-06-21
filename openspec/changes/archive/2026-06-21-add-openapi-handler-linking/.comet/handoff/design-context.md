# Comet Design Handoff

- Change: add-openapi-handler-linking
- Phase: design
- Mode: compact
- Context hash: 2e3629bce5bb06d2a0d53ae4663e61511ea53560b2a312ef80fcbec800b4d862

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/add-openapi-handler-linking/proposal.md

- Source: openspec/changes/add-openapi-handler-linking/proposal.md
- Lines: 1-53
- SHA256: 5f19a4f96e608520917a9ca372e614ee5c6603baf1f8f6d4e10a32be3dad6716

```md
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
```

## openspec/changes/add-openapi-handler-linking/design.md

- Source: openspec/changes/add-openapi-handler-linking/design.md
- Lines: 1-75
- SHA256: 66dcf97fb6163b84a7f4226811cefe02f6b971cff2f8fbf2477564d5e279dca0

```md
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
```

## openspec/changes/add-openapi-handler-linking/tasks.md

- Source: openspec/changes/add-openapi-handler-linking/tasks.md
- Lines: 1-24
- SHA256: f46de30689fe2d3deb9b2c5950bd3684e028837708f61e9486e024abf00245fa

```md
# Tasks — add-openapi-handler-linking

Implementation order. Each task = one focused commit; keep the verification
baseline green (`go build ./...`, `go vet ./...`, `go test ./...`, `mise exec -- task ci`).

## 1. Store linkage
- [ ] 1.1 Add an `http_operation_impls` table (keyed by `index_id` + operation identity) with a migration, mirroring the proto impl-link table.
- [ ] 1.2 Add `HTTPOpImplLink` type and `LinkHTTPOpImpls` writer + reader on `internal/store`, with unit tests (parallel to `LinkRPCImpls`).

## 2. Linker
- [ ] 2.1 Add `MatchOpenAPI` to `internal/link`, reusing `readSourceUnderRoot` and the confidence tiers; define the operation→handler candidate query (operationId/name) and the AST handler-signature confirmation.
- [ ] 2.2 Unit-test `MatchOpenAPI`: AST-confirmed HIGH, name-only fallback tier, no-source fallback, and no-candidate (empty) cases.

## 3. Ingest wiring
- [ ] 3.1 Call `MatchOpenAPI` in `internal/ingest` alongside `MatchRPCs`, gated on `root`, and persist via `LinkHTTPOpImpls`.

## 4. MCP surface
- [ ] 4.1 Extend `ExplainEndpoint` (`internal/mcp/openapi_tools.go`) to read links and return `implemented_by[]` (empty list when none).
- [ ] 4.2 Update the stale limitation strings in `internal/mcp/context_tools.go` (and `get_context` if it surfaces the same note).

## 5. End-to-end + docs
- [ ] 5.1 Extend the e2e (`internal/mcp/e2e_test.go` / `e2e_surface_test.go`) to assert HTTP `implemented_by`, mirroring the proto assertion.
- [ ] 5.2 Reconcile any doc/count drift introduced by the new field (design.md "deferred" note, getting-started, concepts).
- [ ] 5.3 Flip roadmap item 0.2 status (🔎 → ✅) in `Roadmap.md` and update the matching deferred note in `docs/`.
```

## openspec/changes/add-openapi-handler-linking/specs/ast-linking/spec.md

- Source: openspec/changes/add-openapi-handler-linking/specs/ast-linking/spec.md
- Lines: 1-64
- SHA256: 7f17d43f0b7dec8c0da2cf1973a190a033c3203387e01c540d1d16c91d3cae15

```md
## ADDED Requirements

### Requirement: AST-confirmed HTTP handler matching

The linker SHALL match an OpenAPI HTTP operation to its handler symbol in the code
graph and record an `implemented_by` link, mirroring the existing AST-confirmed RPC
implementation matching and reusing the same source-resolution (`root`) and
graceful-fallback behavior.

Candidate discovery SHALL derive handler-name candidates from the operation's
`operationId` only (exact and PascalCase forms); an operation without an
`operationId` SHALL produce no link. The link SHALL be scored on a three-tier
ladder analogous to the RPC linker:

- **HIGH** — a name candidate is confirmed to be an HTTP handler by parsing its
  declaration with `go/ast` (parameters `http.ResponseWriter` and `*http.Request`)
  under `root`; or, when `root` is unavailable, a legacy string-scan of the node's
  `source_file` confirms the same handler shape.
- **MEDIUM** — a name candidate exists and the repo has HTTP route-registration
  **presence** (a coarse, existence-based signal analogous to the RPC
  service-registration check), but the handler signature is not AST-confirmed.
- **LOW** — a name candidate exists by name alone, with neither signature
  confirmation nor route-registration presence.

A node that matches by name but whose AST declaration is not an HTTP handler (for
example a same-named domain-service method) SHALL NOT be promoted to HIGH on the
strength of the name.

#### Scenario: operation confirmed by AST handler signature

- **WHEN** an operation `reserveProduct` (`POST /products/{productId}/reservations`)
  has a candidate handler node whose `go/ast` declaration is an HTTP handler
  (`func (h *Handler) ReserveProduct(w http.ResponseWriter, r *http.Request)`)
  reachable under the repo `root`
- **THEN** the linker records an `implemented_by` link at HIGH confidence with a
  reason indicating AST confirmation

#### Scenario: HIGH via string-scan fallback when root is absent

- **WHEN** the repo sets no `root` but the candidate handler's `source_file` is
  directly readable and its signature matches the HTTP-handler shape
- **THEN** the linker records the `implemented_by` link at HIGH confidence via the
  string-scan fallback

#### Scenario: MEDIUM via route-registration presence

- **WHEN** a name candidate exists and the repo contains HTTP route-registration
  infrastructure, but the candidate's handler signature is not AST-confirmed
- **THEN** the linker records the `implemented_by` link at MEDIUM confidence

#### Scenario: LOW for a same-named non-handler symbol

- **WHEN** the only name candidate is a same-named domain-service method
  `func (s *Service) ReserveProduct(ctx context.Context, req *Request) (*Reservation, error)`
  (not an HTTP handler) and there is no route-registration presence
- **THEN** the linker records the candidate at LOW confidence and does not promote
  it to HIGH

#### Scenario: no operationId or no candidate yields no link

- **WHEN** an operation has no `operationId`, or no node in the code graph matches
  the derived handler name
- **THEN** the linker records no `implemented_by` link for that operation and does
  not error
```

## openspec/changes/add-openapi-handler-linking/specs/openapi-tools/spec.md

- Source: openspec/changes/add-openapi-handler-linking/specs/openapi-tools/spec.md
- Lines: 1-20
- SHA256: f4824c954bee49ef93e27886ba77776aae71157a406d8d3ab1990a7d771be89f

```md
## MODIFIED Requirements

### Requirement: explain_endpoint returns contract data
The system SHALL provide `explain_endpoint` that returns the OpenAPI contract data for a `(method, path)`: summary, operationId, request schema, response schema, security, tags, and spec path. It SHALL additionally return an `implemented_by` field listing the handler symbol(s) linked to the operation by the AST linker, each carrying a confidence tier; when no handler link exists, `implemented_by` SHALL be an empty list (not an error). Existing contract fields SHALL be unchanged, so the addition is backward compatible.

#### Scenario: Contract data returned
- **WHEN** `explain_endpoint` is called with a known method and path
- **THEN** it returns the summary, operation_id, request_schema, response_schema, security, tags, and spec_path

#### Scenario: Handler link returned when implementation is indexed
- **WHEN** `explain_endpoint` is called for an operation whose handler was AST-linked during indexing
- **THEN** the result includes a non-empty `implemented_by` list naming the handler node(s) with a confidence tier

#### Scenario: Empty handler link when no implementation is linked
- **WHEN** `explain_endpoint` is called for an operation with no linked handler
- **THEN** the result includes an empty `implemented_by` list and the contract data is still returned

#### Scenario: Unknown endpoint returns not-found
- **WHEN** `explain_endpoint` is called with a method/path that is not indexed
- **THEN** it returns a structured not-found result, not an error/crash
```

