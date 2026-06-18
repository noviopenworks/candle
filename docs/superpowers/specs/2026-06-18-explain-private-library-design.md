---
comet_change: add-explain-private-library
role: technical-design
canonical_spec: openspec
archived-with: 2026-06-18-add-explain-private-library
status: final
---

# explain_private_library — Technical Design

## Summary

Add `explain_private_library`, a both-sides MCP tool for internal Go libraries: the provider
definition (exports, packages, doc synopsis, defining repo) plus **cross-repo consumer
aggregation** — the dimension `find_library_consumers` explicitly defers. Both sides link to
the code graph: exports → provider nodes (clean), consumer usages → consumer nodes
(best-effort, by enclosing definition). Additive; existing private-library tools are unchanged.

## Architecture

```
explain_private_library (MCP tool, 15th)
   │  registerExplainPrivateLibrary(server.go) — textResult/mustJSON/toolErr
   ▼
Tools.ExplainPrivateLibrary(query) (LibraryExplanation, error)   [SDK-free]
   │
   ├─ resolve: FindPrivateLibraries(query) + FindPrivateDeps(query)  → best + candidates | ErrNotFound
   ├─ provider: PrivateLibraryByModule(modulePath)
   │     └─ export → NodesByLabel(providerIndex, symbol)   (clean link)
   └─ consumers: PrivateConsumersAcrossRepos(modulePath)   [NEW store query, cross-index]
         └─ usage → NodesByFile(consumerIndex, file) → nearest-preceding-line node  (best-effort)
```

## Components

### `internal/store/godep.go` (modified) — the one genuinely new query

`PrivateConsumersAcrossRepos(modulePath string) ([]RepoConsumer, error)`: joins
`private_library_usages` → `indexes` → `repos` (and unions `dependencies` for the pinned
version) filtered by `module_path`, **no `index_id` filter**, grouped per index. Returns per
consuming repo: `IndexID`, `Repo` (org/name), `Commit`, `Version`, `UsedPackages`,
`UsedSymbols []PrivateUsage`. New type `RepoConsumer`.

### `internal/mcp/library_explain.go` (new)

`Tools.ExplainPrivateLibrary` + result types (kept in a new file so `godep_tools.go` stays
focused):

- `LibraryExplanation{ Query, Resolved RepoSummary-like provider id, Provider ProviderInfo,
  Consumers []ConsumerInfo, Candidates []string, Limitations []string }`.
- `ProviderInfo{ ModulePath, Repo, Commit, DocSynopsis, Packages []string, Exports []ExportInfo }`.
- `ExportInfo{ PackagePath, Symbol, Kind, Doc, Node *store.NodeRow, Resolved bool }`.
- `ConsumerInfo{ Repo, Commit, Version, UsedPackages []string, Usages []UsageLink }`.
- `UsageLink{ Usage store.PrivateUsage, Node *store.NodeRow, Resolved bool }`.

Strictly typed where tests assert; `Node` pointers are nil + `Resolved:false` when unlinked.

### `internal/mcp/server.go` (modified)

Add `"explain_private_library"` to `ToolNames` (after `find_library_consumers`) and a thin
`registerExplainPrivateLibrary`.

### Tests & docs

- `internal/store/godep_test.go` — cross-index aggregation test.
- `internal/mcp/library_explain_test.go` — provider+consumers, fuzzy best+candidates,
  provider-less, unknown→not-found, export link, consumer enclosing-node link + unresolved.
- `internal/mcp/e2e_surface_test.go` — 14 → 15.
- `docs/tools.md`, `docs/examples.md`, `README.md`.

## Key Decisions

| ID | Decision | Rationale |
|----|----------|-----------|
| D1 | New cross-index store query | Only missing primitive; everything else reuses existing queries. This change touches `internal/store`. |
| D2 | Fuzzy resolution → best + candidates | Mirrors `resolve_repo`; tolerant input for an explain tool. Exact module-path match preferred. |
| D3 | Reuse global `PrivateLibraryByModule` | Provider lookup is already global; provider-less libs yield empty provider + consumers still returned. |
| D4 | Consumer link = file + nearest-preceding line | `NodesByFile` then greatest `source_location` line ≤ `usage.Line` (enclosing definition); unresolved-marked otherwise. Best-effort, never errors. |
| D5 | Export link = `NodesByLabel(providerIndex, symbol)` | Clean symbol-label match in the defining index. |
| D6 | Lean-typed result, additive tool | Typed entries with explicit `Resolved`/node ref; `limitations` always present. 14 → 15 tools. |

### D4 detail — nearest-preceding line

`source_location` is `L<n>`. For a usage at `file:line`:
1. `nodes = NodesByFile(consumerIndex, usage.File)`.
2. Parse each node's `L<n>` → int; keep nodes with `n ≤ usage.Line`.
3. Link the node with the **greatest** such `n` (the enclosing definition).
4. If `nodes` is empty or none have `n ≤ usage.Line` → `Resolved:false`, `Node:nil`.

## Data Flow

1. Resolve query → module path (+ candidates) or `ErrNotFound`.
2. Provider: global lookup; link each export to a provider node.
3. Consumers: cross-index aggregation; for each usage, best-effort enclosing-node link.
4. Assemble `LibraryExplanation` with `limitations`; marshal via `mustJSON`.

## Error Handling & Boundaries

- Unknown query → `ErrNotFound`.
- Provider-less library → empty provider, consumers still returned, no error.
- Unresolvable links → `Resolved:false` markers, call succeeds.
- `limitations` always non-empty: version-diff/breaking-change deferred; multi-hop/transitive
  dependents deferred; non-Go ecosystems deferred.

## Testing Strategy

TDD, failing-test-first. Seed helper builds a provider index (defines module, exports) and two
consumer indexes (usages at known file:line, with consumer nodes for link tests). Groups: store
aggregation → tool provider+consumers → fuzzy/boundary → graph linking → registration → docs →
verification. Gates: `go test ./...`, `go vet ./...`, diff-scope check.

## Non-Goals

Version-diff / breaking-change analysis, multi-hop call-path expansion, transitive dependents,
non-Go ecosystems, and any change to `find_private_library` / `find_library_consumers` behavior.
