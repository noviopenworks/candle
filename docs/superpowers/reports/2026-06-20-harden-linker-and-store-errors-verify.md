# Verification: harden-linker-and-store-errors

- **Change:** `harden-linker-and-store-errors`
- **Mode:** light
- **Date:** 2026-06-20
- **Branch:** `feature/20260620/harden-linker-and-store-errors`
- **Base ref:** `6be0140`
- **Result:** PASS

## Scope check

| Upgrade criterion | Status |
|---|---|
| New capability | none — hardening only |
| Architecture / interface change | none |
| Delta spec needed | no — no requirement or acceptance scenario changes |
| New tool / resource / schema | none |

The change is correctly scoped as a full-workflow hardening change (it touches
11 files, exceeding the tweak threshold, but introduces no capability).

## Build / static analysis / tests

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | Success |
| Vet | `go vet ./...` | No issues found |
| Tests | `go test ./...` | 116 passed in 12 packages |

Matches the pre-change baseline (116/12). No regressions.

## Defect-class verification (by inspection against the committed diff)

1. **Path traversal (`internal/link/link.go`).** `readSourceUnderRoot` resolves
   `filepath.Abs(root)`, joins + re-abses the target, and rejects when
   `filepath.Rel(absRoot, absPath)` is `".."`, starts with `"../"`, or is
   absolute. Both AST-match call sites (`score`, `astExportPick`) route through
   it; both fall through to "no match" on `ok=false`, matching pre-patch
   behavior on unreadable files. The legacy `signatureMatches` fallback
   retains a `#nosec G304` with rationale. ✓ fixed.

2. **Close-error propagation.**
   - `internal/store/store.go::Open` joins migration/pragma err with `db.Close`. ✓
   - `internal/store/godep.go`: `FindPrivateLibraries`,
     `PrivateLibraryByModule`, `PrivateConsumersAcrossRepos` all propagate
     `rows.Close()` via `errors.Join`; the close-on-success path no longer
     overwrites the scan error. ✓
   - `internal/ingest/ingest.go`: manifest-file `Close` joins its error with
     the parse error. ✓

3. **SQL column interpolation (`internal/store/query.go`).** `edges(col string)`
   is gone; replaced by explicit `Callees` / `Callers` with constant SQL plus a
   `scanEdges` helper typed against a four-method rows interface. No string
   interpolation remains. ✓ fixed.

4. **Gosec annotations.** `internal/godep/{exports,godep,modfile}.go` and
   `internal/openapi/openapi.go` carry `#nosec G304` with one-line rationale at
   manifest-derived read sites. ✓

## Regression surface

The `internal/mcp` e2e test (`TestEndToEndToolSurface`) drives the full
index → link → store → serve path against a fixture graph; it exercises the
changed code paths (linker reads, store query, ingest close) and stays green,
confirming no behavioral regression in the end-to-end flow.

## Out of scope (deferred)

- Enabling the full gosec ruleset as a CI gate (lands with the CI change).
- Fuzzing harness for `readSourceUnderRoot`.
