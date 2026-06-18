## Context

candlegraph indexes private Go libraries from both sides. The provider lookup
`Store.PrivateLibraryByModule(modulePath)` is already **global** (not index-scoped) and
returns the defining index plus exports. Consumer data, however, is index-scoped:
`Store.PrivateUsagesByModule(indexID, modulePath)` and `Store.DependencyByModule` operate
on one index, and `Tools.FindLibraryConsumers` returns a `deferred` marker for the
cross-repo dimension. Fuzzy resolution already exists via `Store.FindPrivateLibraries`
(matches module path / package / synopsis / readme) and `Store.FindPrivateDeps`
(path-only deps). Repo identity comes from `indexes JOIN repos` (see `registry.List`).

`explain_private_library` composes these and adds the one missing piece: a cross-index
consumer aggregation query.

## Goals / Non-Goals

**Goals**
- One additive `Tools.ExplainPrivateLibrary(query)` returning provider + cross-repo consumers.
- Fuzzy resolution to a best-match library with candidates on ambiguity.
- Code-graph linking: exports → provider nodes (clean), usages → consumer nodes (best-effort).
- Explicit `limitations` and per-item unresolved markers.

**Non-Goals (deferred; surfaced in `limitations`)**
- Version-diff / breaking-change analysis (separate tools).
- Multi-hop call-path expansion or transitive dependents.
- Non-Go ecosystems.
- Changing `find_private_library` / `find_library_consumers` behavior.

## Decisions

### D1: New cross-index store query
Add a global aggregation query in `internal/store/godep.go`, e.g.
`PrivateConsumersAcrossRepos(modulePath) ([]RepoConsumer, error)`, that joins
`private_library_usages` (and/or `dependencies`) → `indexes` → `repos` filtered by
`module_path`, grouped by index, returning per-repo: index id, org/name, commit, pinned
version, used packages, used symbols (file/line). This is the one genuinely new store
method; everything else reuses existing queries. **This change touches `internal/store`**
(unlike get_context).

### D2: Fuzzy resolution mirrors resolve_repo's best+candidates shape
Resolve `query` via `FindPrivateLibraries` across indexes (and `FindPrivateDeps` for
path-only). Pick a best match (exact module-path match preferred, else first ranked) and
return remaining matches as candidate module paths. Unknown query → `ErrNotFound`.
*(Resolution detail — exact ranking — is a build-phase concern; the contract is best + candidates.)*

### D3: Provider section reuses the global lookup
Use `PrivateLibraryByModule(modulePath)` for exports, packages, doc synopsis, defining
index. If no provider row exists (consumed-but-undefined), the provider section is empty
and consumers are still returned (D1).

### D4: Code-graph linking
- **Exports → provider nodes (clean):** for each export symbol, `NodesByLabel(providerIndexID, symbol)`;
  attach the resolved node (or mark unresolved).
- **Usages → consumer nodes (best-effort):** a usage names the *provider's* exported symbol
  plus the consumer's `file:line`. The brainstorming phase must choose the matching
  strategy (by consumer `file` + line proximity, by symbol label in the consumer index, or
  a combination). Unresolvable links are marked, never errored. **This is the main design
  risk and the key open question for brainstorming.**

### D5: Additive tool, thin registration
`Tools.ExplainPrivateLibrary` is SDK-free; `server.go` adds `explain_private_library` to
`ToolNames` and a thin `registerExplainPrivateLibrary` using existing
`textResult`/`mustJSON`/`toolErr`. Advertised tool count 14 → 15.

## Risks / Trade-offs

- **Consumer-link semantics (D4):** the usage symbol is the provider's, not the consumer's
  calling function — best-effort matching may yield false/empty links. Mitigated by explicit
  unresolved markers and a brainstorming decision on the matching rule.
- **Surface-count drift:** `e2e_surface_test.go` must move 14 → 15.
- **Cross-index query cost:** acceptable for the current snapshot-store scale; no new indexes
  assumed beyond what exists (`private_library_usages` has module_path).

## Open Questions (for design/brainstorming)
- D4 consumer-link matching strategy (the central design decision).
- Whether to include the provider repo itself when it also consumes the library.
- Result typing depth: which sub-fields are strictly typed vs `any` (lean typed where tests assert).
