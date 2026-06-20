---
comet_change: harden-linker-and-store-errors
role: technical-design
canonical_spec: openspec
status: ready-to-build
---

# Design Doc: harden-linker-and-store-errors

## Context

Security and error-handling sweep of the indexer → linker → store path. Four
defect classes were found in code that handles user-supplied manifest inputs;
patches were prepared in the working tree and attributed to this change per the
Comet dirty-worktree protocol. This doc records the technical rationale so the
change has the deep-design artifact the full workflow requires.

The full proposal and per-file change list live in
`openspec/changes/harden-linker-and-store-errors/{proposal,design,tasks}.md`.

## Goals / non-goals

**Goals**

- Close the path-traversal vector in the AST linker.
- Stop discarding `Close()` errors in store and ingest.
- Remove SQL column-name interpolation from the edges query.
- Document why manifest-derived `os.ReadFile` sites are trusted (gosec).

**Non-goals**

- No new capability, tool, resource, or schema.
- No change to manifest trust boundaries (the manifest remains the trust root).
- No fuzzing harness or new test scaffolding.

## Design

### Path traversal guard

`readSourceUnderRoot(root, sourceFile)` is the single chokepoint for reading
graph-referenced source under a repo root:

1. `filepath.Abs(root)` — canonicalize root.
2. `filepath.Join(absRoot, sourceFile)` then `filepath.Abs` — canonicalize target.
3. `filepath.Rel(absRoot, absPath)` — compute the relative path.
4. Reject if `rel == ".."`, `strings.HasPrefix(rel, "../")`, or
   `filepath.IsAbs(rel)`. These are exactly the escape shapes.
5. `os.ReadFile(absPath)` with `#nosec G304` anchored at the verified path.

Both AST-match call sites (`score`, `astExportPick`) return "no match" when the
helper returns `ok=false`, which is identical to today's behavior on any
unreadable file — so no behavioral regression.

The bare-`source_file` fallback in `signatureMatches` (no root configured) keeps
its `os.ReadFile` with `#nosec G304` and rationale: read-only matching against
graph metadata, unreadable paths simply don't match.

### Close-error propagation

`errors.Join(err, closeErr)` at every site where a `Close` error was discarded.
The bug fix in `PrivateConsumersAcrossRepos` / `FindPrivateLibraries`: the
deferred close-on-success was overwriting the loop's scan error with `nil`; the
fix splits the close-on-error path (join) from the close-on-success path
(propagate scan err).

### SQL column interpolation removal

`edges(col string)` → explicit `Callees` / `Callers` with constant SQL + a
`scanEdges` helper typed against a minimal rows interface. Removes the
interpolation, enables statement reuse, keeps callers readable.

### Gosec annotations and cleanups

Manifest-derived reads get `#nosec G304` with one-line rationale. Two minor
idiomatic cleanups ride along (`(&doc.Package{}).Synopsis`; `append` spread);
`graph/loader.go`'s delete loop over the four code-graph tables is unrolled.

## Verification plan

- `go build ./...`, `go vet ./...`, `go test ./...` stay green (baseline 116
  tests / 12 packages).
- The `internal/mcp` e2e test drives link → store → ingest end-to-end, so
  regressions in the traversal guard or close-error paths surface there.
- No new tests: the guard's contract is "unreadable/escaping paths do not
  match," identical to pre-patch behavior on unreadable files.

## Risk

Low. Every change preserves observable behavior; the patch set was prepared
against the existing test suite and re-verified before commit.
