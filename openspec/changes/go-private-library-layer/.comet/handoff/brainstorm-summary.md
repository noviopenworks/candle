# Brainstorm Summary

- Change: go-private-library-layer
- Date: 2026-06-16
- Status: design CONFIRMED by user (2026-06-16); ready for Design Doc + delta specs

## Confirmed Decisions

1. **Used-symbol resolution = go/ast import+selector heuristic.** Parse consumer `.go`
   files with go/ast (no build/type-check): record imports of private modules, then
   collect `alias.Symbol` selector expressions with file/line. Consistent with the
   parse-files-don't-build approach of the OpenAPI/proto parsers. (Does not reuse the
   code graph for usages — parses source directly.)
2. **find_library_consumers = single-repo scoped.** Returns the queried repo's usage
   of a given module (version + used packages + used symbols). **Cross-repo aggregation
   (who across all repos depends on X) is DEFERRED** to a later change.
3. **go.work INCLUDED.** Parse go.mod (require/replace) + go.sum cross-check + go.work
   (`use` directives, multi-module workspace + local replacement) via golang.org/x/mod/modfile.
4. **Private classification = per-repo prefix list.** Each repo's manifest entry declares
   its own internal module-path prefixes.
5. **Discovery = manifest lists exact go.mod/go.work paths.** Source tree to scan = each
   listed file's directory (walk for *.go beneath it, like proto directory expansion).
6. **find_private_library purpose match = module/package paths + package doc synopsis +
   README.** Match query (case-insensitive substring) against module path, package import
   paths, `// Package x ...` doc synopses (go/ast), and the module's README text.

## Key Trade-offs and Risks

- go/ast heuristic can't perfectly resolve dot-imports/shadowing/aliased re-exports;
  accepted for MVP (documented best-effort). No module download/build required.
- Per-repo private prefixes add manifest boilerplate but avoid global-config coupling.
- lib:// resource scheme is module-path-keyed (per CLAUDE.md), not repo/commit-keyed —
  provider lookup by module_path is a bounded cross-index read (distinct from the deferred
  cross-repo CONSUMER aggregation). To confirm in design presentation.

## Testing Strategy (draft)

- Parser unit tests: go.mod require/replace + indirect flag; go.work use directives;
  go.sum cross-check; exported-symbol extraction (func/type/interface/const/constructor);
  README + package doc synopsis.
- Consumer tests: import + selector usage resolution with file/line; private vs public
  classification (public deps recorded shallow, not deeply indexed).
- Tool tests: find_private_library (path/readme/export match), find_library_consumers
  (single-repo version + used symbols).
- Idempotency: re-index replaces rows cleanly.

## Spec Patches (delta spec to author — none exist yet)

- New `go-dependency-index`: manifest discovery (go.mod/go.work paths + per-repo private
  prefixes), modfile parsing, provider exports, consumer usages (go/ast heuristic), private
  classification, idempotency.
- New `private-library-tools`: find_private_library, find_library_consumers (single-repo;
  cross-repo deferred marker), lib:// resources.

## Resolved in design presentation

- Storage: dedicated index_id-scoped tables `dependencies`, `private_libraries`,
  `private_library_exports` (gains `node_id` for graph link), `private_library_usages`.
- **lib:// stays repo-scoped**: single-index provider lookup of the `private_libraries`
  row by module_path (no cross-repo consumer aggregation; module not defined by an
  indexed repo → not-found). Cross-repo consumer aggregation deferred.
- **Provider exports LINK to Graphify code nodes**: each exported symbol best-effort
  matched to a code node (label == symbol within the provider index, package-file scoped
  when possible); store node_id on private_library_exports. Reuse internal/link package.
- find_library_consumers single-repo; returns deferred marker for cross-repo aggregation.
- Code layout: internal/godep (parser), internal/store/godep.go, internal/mcp/godep_tools.go,
  config per-repo `go: {modules, private_prefixes}` block, internal/link export-matcher,
  ingest wiring after graph.Load (export→node link needs nodes).
- Deps: golang.org/x/mod/modfile; go/ast,go/parser,go/token (stdlib).
