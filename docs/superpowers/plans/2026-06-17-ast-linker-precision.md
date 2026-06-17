---
change: ast-linker-precision
design-doc: docs/superpowers/specs/2026-06-17-ast-linker-precision-design.md
base-ref: 477b557f615273defda18fc960aaf3a755cfde3a
---

# Implementation Plan — ast-linker-precision

Approach 1 (`go/parser`, syntax-only) per the design doc. TDD: a failing test
precedes each implementation step. Go-only. Additive manifest `root`; graceful
fallback to the existing heuristic when source is unavailable.

## Task 1 — Manifest source root (`internal/config`)

1.1 **Test:** a manifest with `root: /abs/repo` loads `RepoConfig.Root`; absent `root` yields empty string.
1.2 **Impl:** add `Root string \`mapstructure:"root"\`` to `RepoConfig`; no validation failure when empty.

Verify: `go test ./internal/config/`.

## Task 2 — AST matcher (`internal/link`)

2.1 **Test:** `astSignatureMatch(root, "server.go", "ReserveProduct", "unary")` returns `(true, true)` for a fixture method `func (s *Server) ReserveProduct(ctx context.Context, req *pb.X) (*pb.Y, error)`.
2.2 **Test:** server-stream fixture `func (s *Server) Sync(req *pb.X, stream pb.Svc_SyncServer) error` → `(true, true)` for `"server_stream"`/default; unary query → `(false, true)`.
2.3 **Test:** multi-line signature fixture matches; wrong-receiver/free function → `(false, true)`; missing/unparseable file → `(_, false)` (ok=false signals fallback).
2.4 **Impl:** `astSignatureMatch` using `go/parser.ParseFile`; walk decls for `*ast.FuncDecl` with matching `Name` and non-nil `Recv`; classify unary vs streaming from `Type.Params`/`Results`.
2.5 **Impl:** thread `root` into `MatchRPCs`; call AST matcher; on `ok=false` fall back to existing `signatureMatches`. Recalibrate `score()` so AST-confirmed → HIGH with reason `...+ast`.

Verify: `go test ./internal/link/`.

## Task 3 — Export disambiguation (`internal/link`)

3.1 **Test:** two nodes named `ValidateToken` in different packages; with `root`, `MatchExports` picks the one whose AST declaration is in the export's package.
3.2 **Impl:** thread `root` into `MatchExports`; when available, confirm the declaring package via AST before falling back to the SourceHint substring heuristic.

Verify: `go test ./internal/link/`.

## Task 4 — Ingest wiring (`internal/ingest`)

4.1 **Test:** ingest resolves a repo's `root` and passes it to the linker (assert links gain the AST tier for a fixture repo with source); a repo without `root` still indexes.
4.2 **Impl:** resolve `root` per repo, pass into `MatchRPCs`/`MatchExports`; warn (don't fail) when absent.

Verify: `go test ./internal/ingest/ ./internal/mcp/` (e2e stays green).

## Task 5 — Full verification

5.1 `go build ./...` · 5.2 `go vet ./...` · 5.3 `go test ./...` (all packages).
5.4 Regression: a repo without `root` produces identical link tiers to pre-change behavior.

## Commit strategy

One commit per task (or per coherent test+impl pair), message reflecting intent.
Check off the OpenSpec `tasks.md` item as each maps complete.
