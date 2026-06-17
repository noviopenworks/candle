---
comet_change: ast-linker-precision
role: technical-design
canonical_spec: openspec
---

# AST-backed linker precision — Technical Design

## Context

`internal/link` decides `implemented_by` for gRPC RPCs and `NodeID` for private
library exports. RPC confirmation today (`signatureMatches`) reads the source with
`os.ReadFile` and scans **line by line** with `strings.Contains` for `rpcName(`,
`context.Context`, and `Server)`. That heuristic:

- breaks on signatures that span multiple lines,
- can be fooled by comments and string literals,
- cannot reason about receiver types or generics,
- only reaches its HIGH-confidence tier when `source_file` is readable from the
  process working directory (its own doc-comment admits this).

The contract→code links are candlegraph's differentiator, so this change makes the
HIGH tier *earned* by real `go/ast` analysis, behind an explicit source root, with
a clean fallback to the current heuristic.

## Approach (Approach 1: go/parser, syntax-only)

Parse only the candidate's source file with `go/parser` (no type checking, no
build graph) and inspect the declaration structurally.

```
   index time, per repo:
   ┌──────────────────────────────────────────────────────────┐
   │ manifest root? ──no──▶ fallback: name/service heuristic    │
   │      │ yes                         (today's tiers)          │
   │      ▼                                                      │
   │ resolve source_file under root                              │
   │      ▼                                                      │
   │ go/parser.ParseFile(file, ParseComments off)                │
   │      ▼                                                      │
   │ find FuncDecl: Name == rpc.Name AND Recv != nil (a method)  │
   │      ▼                                                      │
   │ classify params/returns:                                    │
   │   unary    = (context.Context, *Req) (*Resp, error)         │
   │   stream   = (*Req, <Svc>_<Rpc>Server) error                │
   │   compare to rpc.StreamKind                                 │
   │      │ match → HIGH                                         │
   │      └ no match / not found → fallback tier                 │
   └──────────────────────────────────────────────────────────┘
```

### Alternatives considered

- **Approach 2 — `go/packages` with type info.** Would verify the method truly
  satisfies the generated server interface. Rejected for MVP: needs a buildable
  module plus all dependencies resolvable at index time; heavy, slow, and fails on
  partial checkouts. The marginal precision isn't worth the operational cost here.
- **Approach 3 — hybrid (parser default, opt-in packages).** Deferred. Approach 1
  leaves a clean seam to layer type info later if a future need justifies it.

## Components

| Unit | Responsibility |
|------|----------------|
| `link.astSignatureMatch(root, sourceFile, rpcName, streamKind) (bool, ok)` | Parse + classify one candidate; `ok=false` when source unavailable/unparseable |
| `link.MatchRPCs` | Use AST match when available; else fall back to name/service scoring |
| `link.MatchExports` | Prefer the node whose AST declaration is in the export's package |
| `link.score` | Map signals → tier: AST-confirmed HIGH, name+service MEDIUM, name LOW |
| `config.RepoConfig.Root` | Optional absolute/repo-relative source root |
| `ingest` | Resolve `root`, pass it into the linker |

AST logic stays inside `internal/link`, keeping it unit-testable against fixture
`.go` files and keeping ingest/config free of parsing concerns.

## Data flow

`ingest.Run` → for each repo resolve `root` → call `MatchRPCs(s, indexID, rpcs, root)`
and `MatchExports(s, indexID, exports, root)`. Links are stored exactly as today
(`store.RPCImplLink` with confidence + reason) — **no schema change**.

## Confidence tiers (recalibrated)

| Signal | Tier | Numeric |
|--------|------|---------|
| AST-confirmed signature (receiver + matching unary/stream shape) | HIGH | 0.9 |
| name + `Register<Svc>Server` present, no AST | MEDIUM | 0.6 |
| name only | LOW | 0.3 |

Numbers are unchanged; HIGH is now earned by AST rather than a string scan. The
`MatchReason` string distinguishes `ast` from `name+service`/`name`.

## Error handling

- No `root`, unreadable file, or unparseable Go → `astSignatureMatch` returns
  `ok=false`; the caller keeps the name/service or name-only tier. Never errors,
  never drops a candidate.
- Indexing succeeds whether or not `root` is set (a missing root may emit a warning).

## Testing strategy

TDD — failing test first per task.

- **Unit (`internal/link`):** fixture source files for unary, server-stream,
  client-stream, multi-line signature, wrong-receiver (no match), and
  same-name-different-package export disambiguation; plus an unparseable/missing
  file asserting the fallback tier (regression guard).
- **Integration (`internal/ingest`):** `root` is resolved and threaded through;
  the mcp end-to-end test still passes.
- **Full suite:** `go build ./...`, `go vet ./...`, `go test ./...`.

## Risks / trade-offs

- **No interface-satisfaction proof.** A method with the right name/shape but not
  actually registered could score HIGH. Mitigation: retain the service-registration
  signal as a contributor; document the limit. Approach 3 can close this later.
- **Index-time parse cost.** Parse lazily per matched candidate file; cache parsed
  files within a single index run.
- **`root` drift.** Stale/absent root degrades gracefully to the heuristic.
