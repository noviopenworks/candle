# Tasks: harden-linker-and-store-errors

## 1. Path traversal guard + close-error hardening (link, store, ingest)

- [x] `internal/link/link.go`: extract `readSourceUnderRoot`; route both AST
      read sites through it; `#nosec G304` on the legacy `signatureMatches`
      fallback.
- [x] `internal/store/store.go`: `errors.Join` the close error in `Open`.
- [x] `internal/store/godep.go`: propagate `rows.Close()` errors (joined) across
      `FindPrivateLibraries`, `PrivateLibraryByModule`,
      `PrivateConsumersAcrossRepos`; fix the close-on-success path that
      discarded the loop error.
- [x] `internal/store/query.go`: replace `edges(col)` with explicit
      `Callees` / `Callers` + `scanEdges` helper; no string interpolation.
- [x] `internal/ingest/ingest.go`: `errors.Join` the manifest-file `Close`.

## 2. Gosec annotations and idiomatic cleanups

- [x] `internal/godep/{exports,godep,modfile}.go`: `#nosec G304` on
      manifest-derived reads; `(&doc.Package{}).Synopsis` fix.
- [x] `internal/openapi/openapi.go`: `#nosec G304` on spec read; `append`
      spread cleanup.
- [x] `internal/graph/loader.go`: unroll the constant delete-loop.
- [x] `internal/mcp/e2e_surface_test.go`: gofmt whitespace fix.

## 3. Verify and commit

- [x] `go build ./...`, `go vet ./...`, `go test ./...` green (baseline 116/12).
- [x] Commit with `fix(link,store): harden path traversal and close errors`.
