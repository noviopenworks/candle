---
change: harden-linker-and-store-errors
design-doc: docs/superpowers/specs/2026-06-20-harden-linker-and-store-errors-design.md
build_mode: direct
isolation: branch
tdd_mode: direct
archived-with: 2026-06-20-harden-linker-and-store-errors
---

# Plan: harden-linker-and-store-errors

## Scope

Apply the prepared security/correctness patch set attributed to this change
(per Comet dirty-worktree protocol — the working tree already contains these
diffs; they are not re-done). Build mode is **direct** (no TDD) because the
patch set is a hardening sweep with no new behavior to drive tests from; the
existing 116-test suite (notably `internal/mcp` e2e) is the regression guard.

## Execution

### Task 1 — Path traversal guard + close-error hardening
Files: `internal/link/link.go`, `internal/store/store.go`,
`internal/store/godep.go`, `internal/store/query.go`, `internal/ingest/ingest.go`.

Already present in working tree. Verify the `readSourceUnderRoot` helper is in
`link.go`, the `errors.Join` close paths are in `store.go`/`godep.go`/`ingest.go`,
and `query.go` uses explicit `Callees`/`Callers` + `scanEdges`.

### Task 2 — Gosec annotations and idiomatic cleanups
Files: `internal/godep/{exports,godep,modfile}.go`, `internal/openapi/openapi.go`,
`internal/graph/loader.go`, `internal/mcp/e2e_surface_test.go`.

Already present in working tree. Verify `#nosec G304` annotations are anchored
at the manifest-derived read sites.

### Task 3 — Verify and commit
1. `go build ./...` — expect Success.
2. `go vet ./...` — expect No issues.
3. `go test ./...` — expect 116 passed / 12 packages (baseline).
4. Stage the code changes (not the OpenSpec artifacts, which commit separately
   at archive time per repo convention).
5. Commit: `fix(link,store): harden path traversal and close errors`.

## Verification gate

All three of build/vet/test green before the commit lands. The commit goes onto
`feature/20260620/harden-linker-and-store-errors`.
