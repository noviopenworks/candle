# Tasks — go-private-library-layer

> Refined against design doc `docs/superpowers/specs/2026-06-16-go-private-library-layer-design.md`
> and delta specs. Scope: provider exports + consumer usages (go/ast, no build), single-repo
> find_library_consumers; cross-repo consumer aggregation deferred.

## 1. Storage
- [x] 1.1 Add `dependencies` (ecosystem, is_private, direct), `private_libraries`, `private_library_exports` (with `node_id`), `private_library_usages` tables to `schema.go`
- [x] 1.2 `internal/store/godep.go`: bundle types + `ReplaceGoDeps` (idempotent per index_id); find/lookup queries

## 2. Dependency parsing
- [x] 2.1 Add per-repo `go: { modules, private_prefixes }` block to `RepoConfig` (`internal/config`)
- [x] 2.2 `internal/godep`: parse `go.mod` (require/replace + indirect) and `go.work` (`use`) with `x/mod/modfile`
- [x] 2.3 Cross-check versions via `go.sum` (mismatch → warning)
- [x] 2.4 Per-repo private classification by module-path prefix; public deps shallow

## 3. Provider side (exports)
- [x] 3.1 Extract exported funcs/constructors/types/interfaces/consts/vars via `go/ast` for private modules the repo defines; package doc synopsis + README
- [x] 3.2 Persist to `private_libraries` + `private_library_exports`

## 4. Consumer side (usages)
- [x] 4.1 Detect imports of private modules per repo (longest-prefix match → module+version)
- [x] 4.2 Resolve used symbols via `alias.Symbol` selector scan with file/line → `private_library_usages`

## 5. Graph linking
- [x] 5.1 `internal/link` export matcher: link each export to a code node by label (package-file scoped when possible); store `node_id`; run in `ingest.Run` after `graph.Load`

## 6. Tools
- [x] 6.1 `find_private_library` (module/package path, doc synopsis, README match; path-only for provider-less deps)
- [x] 6.2 `find_library_consumers` (single-repo: version + used packages + used symbols; deferred cross-repo marker)

## 7. Resources
- [x] 7.1 `lib://<module-path>` + `/version/`, `/package/`, `/symbol/` variants (single-index provider lookup)

## 8. Verification
- [ ] 8.1 Parser unit tests: require/replace/indirect, go.work `use`, go.sum cross-check, export extraction, README/doc synopsis
- [ ] 8.2 Consumer tests: import + selector usage resolution with file/line
- [ ] 8.3 Classification: private deep-indexed, public shallow
- [ ] 8.4 Linker: export → code node match; unmatched → null node_id
- [ ] 8.5 Tool/resource tests: find_private_library, find_library_consumers (deferred marker, not-found), lib:// provider lookup
- [ ] 8.6 Idempotency: re-index → identical row counts

> **Cross-repo consumer aggregation — DEFERRED to a future change** (out of scope; `find_library_consumers` returns a deferred marker for the cross-repo dimension). Not a task in this change's scope.
