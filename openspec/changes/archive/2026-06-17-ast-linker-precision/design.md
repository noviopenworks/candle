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
- **`root` drift** (stale path) — unreadable source simply falls back; no hard failure.
