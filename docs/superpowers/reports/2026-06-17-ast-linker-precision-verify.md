# Verification Report — ast-linker-precision

- Date: 2026-06-17
- Mode: full
- Workflow: full
- Base ref: `477b557f615273defda18fc960aaf3a755cfde3a`
- Build: subagent-driven-development, TDD; commits per task

## Result: PASS

## Full verification checks

| # | Check | Result |
|---|-------|--------|
| 1 | All tasks.md complete `[x]` | PASS (0 unchecked) |
| 2 | Implementation matches `design.md` | PASS — Approach 1 (`go/parser` syntax-only), AST authoritative, legacy fallback |
| 3 | Implementation matches Design Doc | PASS — `astSignatureMatch` + recalibrated `score` reviewed against the doc |
| 4 | Capability spec scenarios pass | PASS — each maps to a passing test (below) |
| 5 | proposal.md goals satisfied | PASS — AST-backed precision, manifest `root`, no-regression fallback |
| 6 | delta spec ↔ design doc consistent | PASS — no build-phase spec drift |
| 7 | design docs locatable | PASS — `docs/superpowers/specs/2026-06-17-ast-linker-precision-design.md` |

## Build / test evidence (fresh)

- `go build ./...` → Success, exit 0
- `go vet ./...` → No issues found, exit 0
- `go test -count=1 ./...` → **93 passed in 12 packages**

## Spec scenario → test mapping

| Spec scenario | Covering test |
|---------------|---------------|
| unary RPC confirmed by AST | `TestAstSignatureMatch/unary_matches`, `TestMatchRPCsAST` (HIGH + `ast`) |
| streaming RPC classified by AST | `.../server_stream_matches`, `.../client_stream_matches`, `.../server_stream_as_bidi_matches` |
| multi-line signature matched | `.../multi-line_unary_matches` |
| root enables AST parsing / root optional | `ingest.TestRunLinksRPCWithASTRoot`, `TestRunLinksRPCWithoutRootDegrades` |
| no regression when source missing | `.../empty_root`, `.../missing_file`, `.../unparseable_file` (ok=false), `TestMatchRPCsASTNegativeNoHigh`, `TestRunLinksRPCWithoutRootDegrades` |
| disambiguate same-named exports | `TestMatchExportsASTDisambiguation` |

## Design conformance notes

- `score()` tier logic verified: AST (true,true)→HIGH `+ast`; AST (false,true)→keep name/service tier (AST authoritative, no HIGH); AST (_,false)→legacy string-scan fallback. Numeric tiers (0.9/0.6/0.3) unchanged.
- `go/parser` syntax-only as designed — no `go/packages`, no build graph at index time.
- Documented limit (no true interface-satisfaction proof) retained with the `Register<Svc>Server` signal as a contributor.

## Security

- No hardcoded secrets or new unsafe operations. AST parsing reads source files under the configured `root` — the same class of filesystem read the legacy heuristic already performed.
