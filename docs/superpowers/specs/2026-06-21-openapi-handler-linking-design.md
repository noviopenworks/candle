---
comet_change: add-openapi-handler-linking
role: technical-design
canonical_spec: openspec
---

# OpenAPI handler linking — technical design

Roadmap item 0.2. Make `explain_endpoint` answer *"which handler implements this
endpoint?"* by adding an OpenAPI operation → handler-symbol linker that mirrors the
proven gRPC `MatchRPCs` pipeline end to end.

Canonical requirements live in the OpenSpec delta specs
(`openspec/changes/add-openapi-handler-linking/specs/{ast-linking,openapi-tools}/spec.md`).
This document records the *how*.

## Background

`explain_rpc` returns `implemented_by` because ingest runs `MatchRPCs`
(`internal/link/link.go:36`) → `store.LinkRPCImpls` → `proto_tools.go` reads it
back. The HTTP path stops at the contract: `ExplainEndpoint`
(`internal/mcp/openapi_tools.go:53`) returns the operation only, and
`context_tools.go` still advertises "OpenAPI endpoint implementation linking is not
yet available." The `link` package was deliberately built generic for this
extension (package doc: *"intentionally generic so the OpenAPI handler linker can
adopt it later"*).

## Why HTTP differs from gRPC (the core constraint)

A gRPC method name uniquely identifies the RPC, and its AST signature is distinctive
per stream-kind, so name + AST is both discovery and confirmation. Every HTTP
handler, by contrast, shares the identical signature `(w http.ResponseWriter,
r *http.Request)`. AST can therefore confirm *"this is a handler"* but not *"this is
THE handler for `POST /products/{id}/reservations`."* Disambiguation rests on the
**name** (operationId → handler symbol). Precise path→handler binding (parsing
router-registration call arguments) is router-library-specific and is **deferred**;
this design uses only a coarse route-registration *presence* signal.

## Approach

```
ingest.Run(repo)
  ├─ MatchRPCs(...)     → store.LinkRPCImpls        (exists)
  └─ MatchOpenAPI(...)  → store.LinkHTTPOpImpls     (new)
                                  │
explain_endpoint ──reads──────────┘→ response.implemented_by[]
```

### 1. Linker — `internal/link`

New entry point, parallel to `MatchRPCs`:

```go
// Op is the subset of an HTTP operation the linker needs.
type Op struct {
    Method      string
    Path        string
    OperationID string
}

func MatchOpenAPI(s *store.Store, indexID int64, ops []Op, root string) ([]store.HTTPOpImplLink, error)
```

For each op: derive name candidates, query `s.NodesByLabel`, score each candidate,
emit a link per candidate (ambiguous matches keep their tier, exactly as
`MatchRPCs` does — no dropping/collapsing).

New helpers (mirroring existing ones):

- `handlerNameCandidates(operationId string) []string` — `{operationId, Title(operationId)}`
  deduped. Empty when `operationId == ""` → the op contributes no link.
- `classifyHTTPHandler(fn *ast.FuncDecl) bool` — params flatten (via existing
  `fieldTypes`) to exactly `[http.ResponseWriter, *http.Request]`, no results.
  Reuses `typeName`/selector helpers; sibling of `classifyUnary`.
- `hasRouteRegistration(s, indexID) (bool, error)` — coarse existence check analog
  of `hasServiceRegistration`: looks for routing-infrastructure node labels
  (e.g. `HandleFunc`, `Handle`, `NewServeMux`, `NewRouter`, or a `*Routes`/`registerRoutes`
  setup symbol) via `NodesByLabel`. Computed once per `MatchOpenAPI` call, not per op.
- A string-scan fallback for the handler shape, sibling of `signatureMatches`:
  reads the node's `source_file` and checks for a `func ... Name(... http.ResponseWriter ... *http.Request ...)`
  line when AST is unavailable.

Scoring (`scoreHTTP`), three tiers mirroring `score()` (`link.go:78`):

```
reason = "name";  conf = confLow
if hasRouteRegistration { reason = "name+route"; conf = confMedium }

matched, ok := astHTTPHandlerMatch(root, node.SourceFile, name)   // reuses readSourceUnderRoot
if ok {                       // AST authoritative
    if matched { reason += "+ast";       conf = confHigh }
    return conf, reason
}
if httpSignatureScan(node.SourceFile, name) {  // root absent → legacy scan
    reason += "+signature";   conf = confHigh
}
return conf, reason
```

This makes the AST gate the disambiguator: a same-named domain-service method fails
`classifyHTTPHandler` and stays at LOW/MEDIUM rather than being promoted to HIGH.

### 2. Store — `internal/store`

Parallel to the proto impl-link (`internal/store/proto.go:98`):

```go
type HTTPOpImplLink struct {
    Method      string
    Path        string
    NodeID      string
    Confidence  float64
    MatchReason string
}

func (s *Store) LinkHTTPOpImpls(indexID int64, links []HTTPOpImplLink) error
func (s *Store) HTTPOpImpls(indexID int64, method, path string) ([]HTTPOpImplLink, error)
```

New table, migration mirroring the proto impl-link table, keyed by
`(index_id, method, path)` — the identity `OperationByMethodPath` already uses.
`LinkHTTPOpImpls` replaces an index's HTTP links idempotently (per-repo re-index is
idempotent project-wide).

### 3. Ingest — `internal/ingest`

Where `MatchRPCs` is called: build `[]link.Op` from the indexed operations, call
`MatchOpenAPI(s, indexID, ops, cfg.Root)`, persist via `LinkHTTPOpImpls`. Gated on
`root` for AST the same way RPC linking is; no root still produces LOW/MEDIUM (and
HIGH via string-scan when the source_file path is readable).

### 4. MCP surface — `internal/mcp`

`ExplainEndpoint` reads `HTTPOpImpls` and returns them, mirroring `ExplainRPC`:

```go
type HTTPOpImpl struct {
    Symbol     string `json:"symbol"`      // node label/id
    Confidence string `json:"confidence"`  // HIGH | MEDIUM | LOW (tier from float)
    Reason     string `json:"reason"`
}
// explain_endpoint result gains: ImplementedBy []HTTPOpImpl `json:"implemented_by"`
```

Empty slice when no link. Replace the stale limitation strings in
`context_tools.go` (`contextLimitations()` and the capability note) now that HTTP
linking exists.

## Testing strategy

- **link unit tests** (`internal/link`): HIGH via AST; HIGH via string-scan (no
  root); MEDIUM via route-presence; LOW for same-named domain-service shape; no
  operationId → no link; no candidate → no link.
- **store unit tests**: `LinkHTTPOpImpls` write + `HTTPOpImpls` read, parallel to
  `LinkRPCImpls`/`proto_test.go`.
- **e2e** (`internal/mcp/e2e_test.go` + `e2e_surface_test.go`): add an HTTP handler
  node to the fixture graph with a source file under `root` whose signature is a
  real handler; assert `explain_endpoint` returns `implemented_by` HIGH for
  `reserveProduct`, mirroring the existing proto `implemented_by` assertion.
- Keep the full baseline green: `go build ./...`, `go vet ./...`, `go test ./...`,
  `mise exec -- task ci`.

## Risks / deferred

- **Router-precise binding deferred.** MEDIUM is coarse repo-level routing presence,
  not path→handler. A precise per-router (chi/gin/echo/net-http) binder is a natural
  roadmap follow-on, not in this change.
- **Naming-convention dependency.** Name-based discovery assumes operationId maps to
  the handler symbol; no-operationId is honestly empty. The demo (change
  `add-runnable-demo`) authors its codebase to this convention.
- **Non-unique signatures.** Acknowledged above; the name carries disambiguation.

## Out of scope

gRPC linking (exists), cross-repo `consumed_by` (1.4), multi-hop traversal (1.3),
breaking-change detection, the runnable demo (change `add-runnable-demo`).
