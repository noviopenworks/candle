# Comet Design Handoff

- Change: ast-linker-precision
- Phase: design
- Mode: compact
- Context hash: 949f692a141886298a80453852ccbb45686089ef121a115040f76fea0ed684e2

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/ast-linker-precision/proposal.md

- Source: openspec/changes/ast-linker-precision/proposal.md
- Lines: 1-50
- SHA256: 635f588d9cd3c8637b39a943b4d0022244169fc226c80c08c389168961f892b9

```md
## Why

candle's value is linking contracts back to code. Today that linking
(`internal/link`) decides `implemented_by` for gRPC RPCs with a **line-by-line
string scan** of the source (`signatureMatches`): it matches an RPC to a code
node by label, checks for a `Register<Svc>Server` label, and confirms the impl
by scanning raw source lines for `rpcName(` and substrings like `context.Context`
/ `Server)`. This is fragile — it breaks on multi-line signatures, can be fooled
by comments or strings, cannot reason about receiver types or generics, and only
reaches its HIGH-confidence tier when the source path happens to be readable from
the process working directory. The linking edges are the product's differentiator,
so their trustworthiness matters more than almost anything else.

## What Changes

- Add an **AST-backed matcher** (`go/ast`) used at **index time** to confirm RPC
  implementations by parsing the real method declaration: receiver, parameter
  shapes, return types, and unary-vs-streaming classification.
- Resolve source through a new **optional per-repo `root:` field** in the
  manifest, so AST reliably finds `source_file` paths instead of depending on the
  process CWD.
- Recalibrate confidence tiers to AST-derived signals (an AST-confirmed signature
  is HIGH; name+service without AST is MEDIUM; name-only is LOW).
- **Fall back** to today's string-scan/name-service heuristic when source is
  unavailable, so repos without a `root:` see no regression.
- Apply the same AST confirmation to private-library **export** linking
  (`MatchExports`) so a same-named symbol in the correct package/declaration wins.

## Capabilities

### New Capabilities
- `ast-linking`: AST-backed confirmation of contract→code links (RPC
  implementations and library exports) at index time, with a manifest source
  root and graceful fallback.

### Modified Capabilities
<!-- None. explain_rpc's and the Go export tools' observable contracts are
     unchanged; only the precision/confidence of the underlying links improves,
     and the new manifest field is additive/optional. -->

## Impact

- **Code:** `internal/link` (new AST matcher + recalibrated scoring + fallback);
  `internal/config` (optional `root` field); `internal/ingest` (pass source root
  into linking). No store schema change — links already carry confidence/reason.
- **Config:** new optional `repos[].root` in `manifest.yaml` (backward compatible).
- **Languages:** Go only (matches the existing gRPC-Go impls and Go private-library layer).
- **Out of scope:** new MCP tools (`find_implementations`/`find_references`), live
  LSP/gopls, replacing the Graphify code-graph dependency, OpenAPI handler
  linking, cross-repo `consumed_by`, non-Go ecosystems.
```

## openspec/changes/ast-linker-precision/design.md

- Source: openspec/changes/ast-linker-precision/design.md
- Lines: 1-81
- SHA256: 6e1425a2aac92b4dee2a4fd61ad55de561c1a987340878d5934dd2058c487281

[TRUNCATED]

```md
## Context

`internal/link` decides `implemented_by` for RPCs and `NodeID` for exports.
RPC confirmation today is `signatureMatches`: `os.ReadFile(source_file)` then
`strings.Contains` per line for `rpcName(`, `context.Context`, `Server)`. It is
fragile (multi-line signatures, comments/strings, no receiver/generic reasoning)
and only reaches HIGH confidence when `source_file` is readable from the process
CWD. This change makes the HIGH tier earned by real `go/ast` analysis, behind an
explicit source root, with a clean fallback to the current heuristic.

## High-level approach

```
   index time, per repo:
   ┌──────────────────────────────────────────────────────────┐
   │ manifest root? ──no──▶ fallback: name/service heuristic    │
   │      │ yes                         (today's tiers)          │
   │      ▼                                                      │
   │ resolve source_file under root                              │
   │      │                                                      │
   │      ▼                                                      │
   │ go/parser.ParseFile (syntax only, no type check)            │
   │      │                                                      │
   │      ▼                                                      │
   │ find FuncDecl whose Name == rpc.Name and Recv != nil        │
   │   • receiver present  → it's a method (server impl shape)   │
   │   • inspect params:                                         │
   │       unary    = (context.Context, *Req) (*Resp, error)     │
   │       stream   = (*Req, <Svc>_<Rpc>Server) error            │
   │   • compare against rpc.StreamKind                          │
   │      │ match → HIGH                                         │
   │      └ no match / not found → fallback tier                 │
   └──────────────────────────────────────────────────────────┘
```

## Decisions and rationale

- **`go/parser` (syntax-only), not `go/packages`.** Parsing single files gives us
  receiver + parameter/return *shapes* — enough to classify unary vs streaming and
  confirm the method is a server impl — without needing a buildable module, its
  dependencies, or network/module resolution. `go/packages` would add real type
  resolution (true interface-satisfaction) but at large cost (build graph, deps)
  for marginal gain here. If a future change needs genuine interface satisfaction,
  it can layer `go/packages` on top; this change deliberately stays at the syntax tier.
- **Manifest `root` is optional and additive.** No existing manifest breaks; repos
  opt in to AST precision by providing source.
- **Fallback preserves behavior.** The existing name/service scoring stays as the
  degraded path, so repos without source see no regression — this is why the change
  is additive, not a `MODIFIED` to `explain_rpc`'s contract.
- **AST confined to `internal/link`.** Ingest passes a resolved source root in;
  parsing logic stays in the link package, keeping the matcher unit-testable with
  fixture source files.

## Confidence tiers (recalibrated)

| Signal | Tier |
|--------|------|
| AST-confirmed signature (receiver + matching unary/stream shape) | HIGH (0.9) |
| name + `Register<Svc>Server` present, no AST | MEDIUM (0.6) |
| name only | LOW (0.3) |

The numeric tiers are unchanged; what changes is that HIGH is now *earned by AST*,
not by a brittle string scan.

## Testing strategy

- **Unit (link):** fixture `.go` files exercising unary, server-stream,
  client-stream, multi-line signatures, wrong receiver, same-name-different-package
  exports, and an unparseable file (fallback). Assert tier + reason.
- **Regression:** a repo with no `root` reproduces today's tiers exactly.
- **Integration:** `ingest` wires `root` through and the mcp e2e still passes.
- Full `go build/vet/test ./...`.

## Risks / trade-offs

- **Syntax-only AST can't prove interface satisfaction** — a method with the right
  name/shape but not actually registered could still score HIGH. Mitigation: keep
  the service-registration signal as a contributing factor; document the limit.
- **Index-time parsing cost** on large repos — parse lazily per matched candidate
  file, not the whole tree; cache parsed files within a run.
```

Full source: openspec/changes/ast-linker-precision/design.md

## openspec/changes/ast-linker-precision/tasks.md

- Source: openspec/changes/ast-linker-precision/tasks.md
- Lines: 1-37
- SHA256: 17f605589411956b6186be8258704484d83981dcc3a4cfd95af2e41bcc760e42

```md
# Tasks — ast-linker-precision

> Go-only. AST at index time via `go/parser` (syntax-only). Additive manifest
> `root`; fallback to today's heuristic when source is unavailable. TDD: each
> task starts with a failing test.

## 1. Manifest source root

- [ ] 1.1 Add optional `root` field to `RepoConfig` in `internal/config` (+ test)
- [ ] 1.2 Validate `root` (absolute or repo-relative); empty is allowed

## 2. AST matcher in internal/link

- [ ] 2.1 Add a source-root parameter to the link entrypoints (`MatchRPCs`, `MatchExports`)
- [ ] 2.2 Implement `astSignatureMatch`: parse the candidate file with `go/parser`, find the `FuncDecl` (Name == rpc, receiver present), classify unary vs streaming from params/returns
- [ ] 2.3 Recalibrate `score`: AST-confirmed → HIGH; name+service → MEDIUM; name → LOW; reason string reflects AST vs heuristic
- [ ] 2.4 Fallback path: when root absent / file unreadable / unparseable, use existing string-scan/name-service tiers (no regression)
- [ ] 2.5 AST-confirm `MatchExports`: prefer the node whose declaration is in the export's package

## 3. Ingest wiring

- [ ] 3.1 Resolve each repo's source root and pass it into the linker in `internal/ingest`
- [ ] 3.2 Keep indexing successful when `root` is absent (warn, don't fail)

## 4. Tests

- [ ] 4.1 Unit fixtures: unary, server-stream, client-stream, multi-line signature, wrong receiver
- [ ] 4.2 Unit: same-name-different-package export disambiguation
- [ ] 4.3 Unit: unparseable/missing source → fallback tier (regression guard)
- [ ] 4.4 Integration: ingest passes `root`; mcp e2e still green

## 5. Verification

- [ ] 5.1 `go build ./...` passes
- [ ] 5.2 `go vet ./...` passes
- [ ] 5.3 `go test ./...` passes (all packages)
- [ ] 5.4 A repo without `root` produces identical link tiers to pre-change behavior
```

## openspec/changes/ast-linker-precision/specs/ast-linking/spec.md

- Source: openspec/changes/ast-linker-precision/specs/ast-linking/spec.md
- Lines: 1-69
- SHA256: bb61863f736c830409b383e7a2907453545f61d80d1943f2decb21ccc5e81c47

```md
# ast-linking

## ADDED Requirements

### Requirement: AST-confirmed RPC implementation matching

The linker SHALL, when a repo's Go source is available, confirm a gRPC RPC's
implementation by parsing the candidate method's declaration with `go/ast` —
inspecting the receiver, parameter types, and return types — rather than scanning
raw source lines. An AST-confirmed match SHALL be recorded at HIGH confidence.

#### Scenario: unary RPC confirmed by AST

- **WHEN** an RPC `ReserveProduct` (unary) has a candidate method
  `func (s *Server) ReserveProduct(ctx context.Context, req *pb.ReserveProductRequest) (*pb.Reservation, error)`
- **THEN** the linker records an `implemented_by` link at HIGH confidence with a
  reason indicating AST signature confirmation

#### Scenario: streaming RPC classified by AST

- **WHEN** an RPC `Sync` (server stream) has a candidate method
  `func (s *Server) Sync(req *pb.SyncRequest, stream pb.InventoryService_SyncServer) error`
- **THEN** the linker classifies it as streaming, matches it against the RPC's
  `stream_kind`, and records the link at HIGH confidence

#### Scenario: multi-line signature matched

- **WHEN** a candidate method's signature spans multiple source lines
- **THEN** the AST matcher still confirms it (where a line-based string scan would fail)

### Requirement: Source resolution via manifest root

The manifest SHALL accept an optional per-repo `root` field giving the absolute
path to the repo's source tree. The linker SHALL resolve a node's `source_file`
against this root for AST parsing.

#### Scenario: root enables AST parsing

- **WHEN** a repo entry sets `root: /abs/path/repo` and a node's `source_file` is `internal/grpc/server.go`
- **THEN** the linker parses `/abs/path/repo/internal/grpc/server.go` with `go/ast`

#### Scenario: root is optional

- **WHEN** a repo entry omits `root`
- **THEN** indexing still succeeds and linking proceeds via the fallback path

### Requirement: Graceful fallback without source

When source is unavailable (no `root`, unreadable file, or unparseable Go), the
linker SHALL fall back to the existing name + service-registration heuristic and
record the link at its corresponding non-HIGH tier, without error and without
dropping the candidate.

#### Scenario: no regression when source is missing

- **WHEN** a repo has no reachable source for a matched RPC
- **THEN** the linker still produces the `implemented_by` candidate at the
  name/service (MEDIUM) or name-only (LOW) tier, exactly as before this change

### Requirement: AST-confirmed export linking

When source is available, private-library **export** linking SHALL prefer the
code node whose AST declaration matches the exported symbol in the expected
package, rather than the first same-named node.

#### Scenario: disambiguate same-named exports

- **WHEN** two code nodes share the symbol name `ValidateToken` in different packages
- **THEN** the linker selects the node whose AST declaration is in the export's package
```

