# Comet Design Handoff

- Change: harden-linker-and-store-errors
- Phase: design
- Mode: compact
- Context hash: 37484914c4c63312fce846f0c67c9e72daec6eb2b76b69f3c85d6e1abc4f4213

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/harden-linker-and-store-errors/proposal.md

- Source: openspec/changes/harden-linker-and-store-errors/proposal.md
- Lines: 1-65
- SHA256: 2dd4f363aef8af98aa13ba0e2ea9a62fcdeba08726fffa5909b3e117984303b9

```md
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
```

## openspec/changes/harden-linker-and-store-errors/design.md

- Source: openspec/changes/harden-linker-and-store-errors/design.md
- Lines: 1-70
- SHA256: aefc80786ae8df06148fca43ce962cae74ad01678ee0c51723528e9341069604

```md
# Design: harden-linker-and-store-errors

This is a hardening change, not a feature. The design fixes four defect classes
without altering observable behavior. See `proposal.md` for the rationale.

## Approach

### 1. Path traversal guard (`internal/link`)

Introduce one helper used by every "read source under a repo root" site:

```go
func readSourceUnderRoot(root, sourceFile string) (path string, src []byte, ok bool)
```

It canonicalizes `root` to absolute, joins `sourceFile`, canonicalizes the
result, and computes `filepath.Rel(absRoot, absPath)`. If the relative path is
`".."`, starts with `"../"`, or is absolute, the path escapes the root and the
helper returns `ok=false` (callers fall back to "no match", matching today's
behavior on any unreadable file). The `#nosec G304` is anchored at the
verified `os.ReadFile(absPath)` with a comment pointing at the guard.

The legacy `signatureMatches` fallback that reads a bare `source_file` (used
when no root is configured) keeps `os.ReadFile(sourceFile)` with a `#nosec
G304` and rationale: that path is read-only matching against graph metadata,
and unreadable paths simply do not match.

### 2. Close-error propagation (`internal/store`, `internal/ingest`)

`errors.Join` is used at every site where a `Close` error was previously
discarded. The pattern:

```go
if closeErr := rows.Close(); closeErr != nil {
    return nil, errors.Join(err, closeErr)
}
```

In `PrivateConsumersAcrossRepos` and `FindPrivateLibraries`, the
close-on-success path is split from the close-on-error path so the loop's scan
error is no longer overwritten by a successful close.

### 3. SQL column interpolation removal (`internal/store/query.go`)

The shared `edges(col string)` helper built the query with
`` `SELECT ... WHERE index_id=? AND ` + col + `=?` ``.
Replace with two named methods (`Callees`, `Callers`) whose SQL is a constant,
and a small `scanEdges(rows)` helper typed against a four-method `rows`
interface. This removes the interpolation, lets the driver reuse the prepared
statement, and keeps callers readable.

### 4. Gosec annotations and idiomatic cleanups

Manifest-derived `os.ReadFile` / `os.Open` sites in `godep` and `openapi` get
`#nosec G304` with a one-line rationale ("explicit user manifest inputs"). Two
minor cleanups ride along: `(&doc.Package{}).Synopsis` instead of allocating
via `doc.Synopsis` from a nil receiver, and `append(dst, src...)` instead of
the element-wise loop. `internal/graph/loader.go`'s delete-loop over the four
code-graph tables is unrolled to four explicit `DELETE` statements for
readability.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` stay green (baseline: 116
  tests across 12 packages).
- The e2e test in `internal/mcp` exercises link, store, and ingest end-to-end,
  so any behavioral regression in the traversal guard or close-error paths
  surfaces there.
- No new tests: the guard's contract ("unreadable / escaping paths do not
  match") is exactly the pre-patch behavior on any unreadable file.
```

## openspec/changes/harden-linker-and-store-errors/tasks.md

- Source: openspec/changes/harden-linker-and-store-errors/tasks.md
- Lines: 1-29
- SHA256: d42eb1e62b0b4b9d5716f52cca04054c71a6d72f33bef0564e84cc17aeac65bd

```md
# Tasks: harden-linker-and-store-errors

## 1. Path traversal guard + close-error hardening (link, store, ingest)

- [ ] `internal/link/link.go`: extract `readSourceUnderRoot`; route both AST
      read sites through it; `#nosec G304` on the legacy `signatureMatches`
      fallback.
- [ ] `internal/store/store.go`: `errors.Join` the close error in `Open`.
- [ ] `internal/store/godep.go`: propagate `rows.Close()` errors (joined) across
      `FindPrivateLibraries`, `PrivateLibraryByModule`,
      `PrivateConsumersAcrossRepos`; fix the close-on-success path that
      discarded the loop error.
- [ ] `internal/store/query.go`: replace `edges(col)` with explicit
      `Callees` / `Callers` + `scanEdges` helper; no string interpolation.
- [ ] `internal/ingest/ingest.go`: `errors.Join` the manifest-file `Close`.

## 2. Gosec annotations and idiomatic cleanups

- [ ] `internal/godep/{exports,godep,modfile}.go`: `#nosec G304` on
      manifest-derived reads; `(&doc.Package{}).Synopsis` fix.
- [ ] `internal/openapi/openapi.go`: `#nosec G304` on spec read; `append`
      spread cleanup.
- [ ] `internal/graph/loader.go`: unroll the constant delete-loop.
- [ ] `internal/mcp/e2e_surface_test.go`: gofmt whitespace fix.

## 3. Verify and commit

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` green (baseline 116/12).
- [ ] Commit with `fix(link,store): harden path traversal and close errors`.
```

