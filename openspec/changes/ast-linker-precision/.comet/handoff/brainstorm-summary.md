# Brainstorm Summary

- Change: ast-linker-precision
- Date: 2026-06-17

## Confirmed Technical Approach

Approach 1 — `go/parser`, syntax-only, single-file. At index time, parse the
candidate's `source_file`, find the `FuncDecl` whose name matches the RPC and
whose receiver is non-nil, and classify from the param/return AST:
- unary:     `(ctx context.Context, *Req) (*Resp, error)`
- streaming: `(*Req, <Svc>_<Rpc>Server) error`

No type checking, no build graph. Rejected Approach 2 (`go/packages` type info)
as too heavy at index time; Approach 3 (hybrid) deferred as a future option.

- New `astSignatureMatch(root, sourceFile, rpcName, streamKind)` in `internal/link`.
- `MatchExports` gains package-aware AST confirmation.
- `score()` recalibrated: AST-confirmed → HIGH, name+service → MEDIUM, name → LOW.
- `internal/config` optional `root`; `internal/ingest` resolves and passes it.
- Source unavailable → existing name/service heuristic fallback (no regression).

## Key Trade-offs and Risks

- Syntax-only AST can't prove interface satisfaction; mitigated by keeping the
  `Register<Svc>Server` signal and documenting the limit.
- Index-time parse cost; mitigated by lazy per-candidate parsing + within-run cache.
- `root` drift / unreadable source → graceful fallback, no hard failure.

## Testing Strategy

TDD. Link-package unit fixtures: unary, server-stream, client-stream, multi-line
signature, wrong receiver, same-name-different-package export, unparseable→fallback.
Ingest integration test for `root` plumbing. mcp e2e stays green. Full
`go build/vet/test ./...`.

## Spec Patches

None. The `ast-linking` delta spec already carries the acceptance scenarios.
