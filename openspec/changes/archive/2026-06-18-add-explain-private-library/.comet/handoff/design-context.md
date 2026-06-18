# Comet Design Handoff

- Change: add-explain-private-library
- Phase: design
- Mode: compact
- Context hash: 0582bed70fc88c9aa9ef3b76366aa40749eceaeca360ff5e76c811ac22ffe1e5

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/add-explain-private-library/proposal.md

- Source: openspec/changes/add-explain-private-library/proposal.md
- Lines: 1-55
- SHA256: 950c5366a881b4691add979d90b03d2f47827c6f6b852d00cb7846cb2755fd51

```md
## Why

candlegraph can locate an internal library (`find_private_library`) and report a single
repo's usage of it (`find_library_consumers`), but the latter explicitly **defers
cross-repo consumer aggregation** — it returns a `deferred` marker for the cross-repo
dimension. So no tool answers "who across the whole org uses this internal library, at
which versions, and which of its symbols?" That cross-repo, both-sides view is the whole
point of indexing a private library from both provider and consumer sides, and the
`get_context` facade lists the same gap as a known limitation.

## What Changes

- Add a new MCP tool `explain_private_library` that explains an internal Go library from
  **both sides in one call**:
  - **Provider side:** resolve the library globally, return its packages, exports
    (package / symbol / kind / doc), doc synopsis, and the defining repo/commit; link each
    export to its **provider code-graph node**.
  - **Consumer side:** aggregate across **all** indexed repos — for each consuming repo:
    pinned version, used packages, and used symbols with file:line; **best-effort** link
    each usage to a **consumer code-graph node**.
  - **Input:** a fuzzy `query` (name / module path / synopsis / readme) resolving to a
    best-match library, returning candidates when ambiguous (mirrors `resolve_repo`).
  - Every response carries explicit `limitations` for deferred behavior and an explicit
    marker for any usage whose consumer code-graph node could not be resolved.
- Add a cross-index store query that aggregates private-library usages/dependencies by
  module path across all indexes, joined to repo identity.
- Register `explain_private_library` as the 15th advertised MCP tool.
- Update docs (`docs/tools.md`, `docs/examples.md`, `README.md`).

Non-breaking: `find_private_library` and `find_library_consumers` keep their current
behavior (the latter keeps its single-repo scope and deferred marker); `explain_private_library`
is the tool that delivers the cross-repo dimension.

## Capabilities

### New Capabilities
<!-- None; this extends the existing private-library-tools capability. -->

### Modified Capabilities
- `private-library-tools`: ADD a requirement for `explain_private_library` — a both-sides,
  cross-repo library explanation with code-graph linking. Existing requirements for
  `find_private_library` and `find_library_consumers` are unchanged.

## Impact

- **New code:** cross-index aggregation query in `internal/store/godep.go`;
  `Tools.ExplainPrivateLibrary` + result types in `internal/mcp` (likely
  `internal/mcp/godep_tools.go` or a new `internal/mcp/library_explain.go`); MCP
  registration in `internal/mcp/server.go` (`ToolNames` + register fn); surface-count
  update in `internal/mcp/e2e_surface_test.go`.
- **Reused:** `PrivateLibraryByModule` (provider), `FindPrivateLibraries`/`FindPrivateDeps`
  (fuzzy resolution), `NodesByLabel` (graph linking), registry repo identity.
- **Docs:** `docs/tools.md`, `docs/examples.md`, `README.md` (tool count 14 → 15).
- **No change** to parsers, ingestion, or existing tool outputs.
- **Dependencies:** none added.
```

## openspec/changes/add-explain-private-library/design.md

- Source: openspec/changes/add-explain-private-library/design.md
- Lines: 1-77
- SHA256: b7e6a0690d56f5fd49f197413a22625235efb1e98139babbbfe30c2496374004

```md
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
```

## openspec/changes/add-explain-private-library/tasks.md

- Source: openspec/changes/add-explain-private-library/tasks.md
- Lines: 1-51
- SHA256: 30782816ce4c2e551538873cec915ab27ecadefcc4ce38fa1071267b1a423b25

```md
# Tasks: add-explain-private-library

> TDD-oriented: each implementation group is preceded by a failing test. Cross-repo
> consumer aggregation is the core new capability; code-graph linking (esp. consumer-side)
> is resolved during design brainstorming.

## 1. Cross-index consumer aggregation (store, test-first)

- [ ] 1.1 Add a failing store test for cross-index aggregation by module path (seed 2 indexes consuming the same private module + a provider index)
- [ ] 1.2 Run the store test and confirm it fails (undefined aggregation method)
- [ ] 1.3 Implement the cross-index aggregation query in `internal/store/godep.go` joining `private_library_usages`/`dependencies` → `indexes` → `repos` by `module_path`, returning per-repo identity, version, used packages, used symbols
- [ ] 1.4 Run the store test and confirm it passes

## 2. ExplainPrivateLibrary provider + consumer aggregation (test-first)

- [ ] 2.1 Add a failing test for `Tools.ExplainPrivateLibrary`: provider exports + cross-repo consumers for a known library
- [ ] 2.2 Run and confirm it fails (undefined `ExplainPrivateLibrary`/result types)
- [ ] 2.3 Implement `Tools.ExplainPrivateLibrary` + result types: fuzzy resolution (best + candidates), provider section via `PrivateLibraryByModule`, consumer aggregation via the new store query, `limitations`
- [ ] 2.4 Run and confirm it passes

## 3. Fuzzy resolution and boundary behavior (test-first)

- [ ] 3.1 Add failing tests: ambiguous query → best + candidates; provider-less library → consumers only, no error; unknown query → `ErrNotFound`
- [ ] 3.2 Run and confirm the new tests fail where expected
- [ ] 3.3 Implement disambiguation + provider-less handling
- [ ] 3.4 Run and confirm all pass

## 4. Code-graph linking (test-first)

- [ ] 4.1 Add failing tests: export → provider node link resolves; unresolved consumer usage is marked (per the brainstorming-decided matching rule)
- [ ] 4.2 Run and confirm they fail
- [ ] 4.3 Implement export→provider-node linking (`NodesByLabel` in provider index) and best-effort consumer usage→node linking with explicit unresolved markers
- [ ] 4.4 Run and confirm all pass

## 5. MCP registration and surface

- [ ] 5.1 Add `"explain_private_library"` to `ToolNames` and register via `registerExplainPrivateLibrary` in `internal/mcp/server.go`
- [ ] 5.2 Update `internal/mcp/e2e_surface_test.go` advertised count/comments 14 → 15
- [ ] 5.3 Run `go test ./internal/mcp -v` and confirm pass

## 6. Documentation

- [ ] 6.1 Update `docs/tools.md`: 15 tools, add `explain_private_library` reference (args, request/response shape) in the private-library section
- [ ] 6.2 Update `docs/examples.md`: add a cross-repo "who consumes this library across the org?" example
- [ ] 6.3 Update `README.md`: tool count 14 → 15

## 7. Final verification

- [ ] 7.1 Run `go test ./...` and confirm pass
- [ ] 7.2 Run `go vet ./...` and confirm pass
- [ ] 7.3 Inspect `git diff` and confirm scope matches the plan
```

## openspec/changes/add-explain-private-library/specs/private-library-tools/spec.md

- Source: openspec/changes/add-explain-private-library/specs/private-library-tools/spec.md
- Lines: 1-71
- SHA256: c5a40f2de69d7cc8aada59ac6b23a1b1bcb0d752823ec1ced5a7187c0b6ab493

```md
# private-library-tools Specification

## ADDED Requirements

### Requirement: explain_private_library explains a library from both sides
The system SHALL provide `explain_private_library` that, given a fuzzy `query` resolving to
a single internal Go library, returns a both-sides explanation: the provider definition and
the cross-repo consumer aggregation. It SHALL be additive — `find_private_library` and
`find_library_consumers` retain their current behavior. Every response SHALL include an
explicit `limitations` list for deferred behavior.

#### Scenario: Provider and cross-repo consumers in one call
- **WHEN** `explain_private_library` is called with a query resolving to an internal library
  that is defined by one indexed repo and consumed by others
- **THEN** it returns the provider definition (module path, packages, exports, doc synopsis,
  defining repo and commit) together with a consumer list where each entry is a consuming
  repo with its pinned version, used packages, and used symbols (with file and line)

#### Scenario: Unknown query returns not-found
- **WHEN** `explain_private_library` is called with a query that matches no indexed library
  or private dependency
- **THEN** it returns a structured not-found result (`ErrNotFound`), not a crash

### Requirement: Fuzzy resolution with candidate disambiguation
`explain_private_library` SHALL resolve its `query` against private library module paths,
package paths, doc synopsis, and README text (case-insensitive). When exactly one library
matches it SHALL explain that library. When multiple libraries match it SHALL select a
best match and also return the other matches as candidates so the caller can disambiguate.

#### Scenario: Ambiguous query returns best match plus candidates
- **WHEN** the query matches more than one internal library
- **THEN** the result explains the best-match library and lists the remaining matches as
  candidate module paths

### Requirement: Cross-repo consumer aggregation across all indexes
`explain_private_library` SHALL aggregate consumers across all indexed repositories, not a
single index. For a resolved module path it SHALL find every index whose dependencies or
private-library usages reference that module, and report each as a distinct consuming repo
with its pinned version and used symbols. This requirement supersedes the deferred cross-repo
marker that `find_library_consumers` returns, without changing `find_library_consumers`.

#### Scenario: Multiple consuming repos are aggregated
- **WHEN** two or more indexed repos depend on the same private module
- **THEN** the consumer list contains one entry per consuming repo, each with that repo's
  identity, pinned version, and used symbols

#### Scenario: Library consumed without an indexed provider still explains consumers
- **WHEN** the resolved module is consumed by indexed repos but no indexed repo defines it
  (no provider exports)
- **THEN** the provider section is empty and the consumer aggregation is still returned,
  without error

### Requirement: Code-graph linking for exports and usages
`explain_private_library` SHALL link provider exports to provider code-graph nodes and
SHALL best-effort link consumer usages to consumer code-graph nodes. When a link cannot be
resolved, the result SHALL mark that item as unresolved rather than failing the call.

#### Scenario: Export links to a provider code-graph node
- **WHEN** a provider export's symbol matches a code-graph node in the provider repo's index
- **THEN** that export carries a reference to the resolved provider node

#### Scenario: Consumer usage links to the enclosing consumer node
- **WHEN** a consumer usage occurs in a file that has code-graph nodes, and at least one node's
  definition line is at or before the usage line
- **THEN** that usage carries a reference to the node with the greatest definition line at or
  before the usage line (the enclosing definition)

#### Scenario: Unresolved consumer link is marked, not errored
- **WHEN** a consumer usage cannot be matched to a consumer code-graph node
- **THEN** that usage is returned with an explicit unresolved marker and the call still
  succeeds
```

