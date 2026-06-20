## Why

A security and error-handling sweep of the indexer/linker/storage path surfaced
four classes of defect in code that runs against user-supplied manifest inputs:

1. **Path traversal in the AST linker.** `internal/link` joined a manifest repo
   root with a graph-supplied `source_file` and called `os.ReadFile` on the
   result without verifying the joined path stayed under the root. A malicious
   or malformed graph could read arbitrary files outside the repo.
2. **Dropped close errors.** Several `rows.Close()` / `db.Close()` / `f.Close()`
   call sites discarded the close error, masking corruption and partial writes
   (`internal/store`, `internal/ingest`). At least one site in
   `PrivateConsumersAcrossRepos` returned `nil` after a scan error because the
   deferred `rows.Close()` branch overwrote the loop error.
3. **SQL column interpolation.** `internal/store/query.go` interpolated a column
   name (`"source"` / `"target"`) into the edges query via a `+col+` concat.
   The value was internal and safe today, but the pattern is a latent injection
   smell and defeats prepared-statement reuse.
4. **Ungoverned `os.ReadFile`.** `gosec` flags every `os.ReadFile` whose path is
   not literally a string literal. The flagged paths are all manifest-derived
   (OpenAPI files, `go.mod`/`go.sum`, README), so they are trusted inputs — but
   the rationale was undocumented.

## What Changes

- **`internal/link/link.go`** — extract `readSourceUnderRoot`, which canonicalizes
  the root, resolves the joined path, and verifies via `filepath.Rel` that the
  result stays under the root (rejecting `..` escapes and absolute paths) before
  reading. Used by both AST-match call sites. The legacy fallback that reads a
  raw `source_file` (no root) gets a `#nosec G304` with rationale.
- **`internal/store/store.go`** — `Open` joins the migration/pragma error with
  the `db.Close` error via `errors.Join` instead of silently dropping it.
- **`internal/store/godep.go`** — every `rows.Close()` in
  `FindPrivateLibraries`, `PrivateLibraryByModule`, and
  `PrivateConsumersAcrossRepos` propagates its error (joined with any in-flight
  scan error); fixes the bug where the close-on-success path discarded the
  loop's error.
- **`internal/store/query.go`** — replace the `edges(col string)` helper with
  explicit `Callees` / `Callers` methods (no string interpolation) and a
  `scanEdges` helper over a minimal `rows` interface.
- **`internal/ingest/ingest.go`** — the manifest-file `Close` during the
  per-repo parse loop joins its error with the parse error.
- **`internal/godep/{exports,godep,modfile}.go`** and
  **`internal/openapi/openapi.go`** — add `#nosec G304` annotations with
  rationale on the manifest-derived `os.ReadFile` / `os.Open` call sites; minor
  idiomatic cleanups (`(&doc.Package{}).Synopsis`, `append` spread).
- **`internal/graph/loader.go`** — unroll the table-name loop into explicit
  `DELETE` statements (clarity; the loop iterated a constant list).
- **`internal/mcp/e2e_surface_test.go`** — gofmt whitespace fix.

## Capabilities

### Modified Capabilities
<!-- None. This is a hardening change: no observable behavior, tool I/O,
     resource shape, or schema changes. The 116-test suite stays green. -->

## Impact

- **Code:** `internal/link`, `internal/store`, `internal/ingest`,
  `internal/godep`, `internal/openapi`, `internal/graph`.
- **Tests:** no new tests; the existing e2e suite (`internal/mcp`) and package
  tests continue to exercise the changed paths.
- **No change** to tool surface, resource URIs, storage schema, or public API.
- **Out of scope:** further gosec rule enablement, fuzzing harnesses, and any
  change to manifest trust boundaries (the manifest remains the trust root).
