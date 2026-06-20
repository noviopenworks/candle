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
