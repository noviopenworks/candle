# Brainstorm Summary

- Change: add-explain-private-library
- Date: 2026-06-18

## Confirmed Technical Approach

Additive `Tools.ExplainPrivateLibrary(query) (LibraryExplanation, error)`, SDK-free; thin
`registerExplainPrivateLibrary` in `server.go` (advertised tools 14 → 15).

- **Resolution:** fuzzy via `FindPrivateLibraries` (+ `FindPrivateDeps` for path-only) across
  the store; best match (exact module-path preferred, else top-ranked) plus remaining matches
  as candidate module paths; no match → `ErrNotFound`.
- **Provider section:** `PrivateLibraryByModule(modulePath)` → packages, exports, doc synopsis,
  defining repo/commit. Each export links to a provider node via
  `NodesByLabel(providerIndex, export.Symbol)` (unresolved-marked otherwise). Empty when provider-less.
- **Consumer section (new capability):** new store query `PrivateConsumersAcrossRepos(modulePath)`
  joining `private_library_usages`/`dependencies` → `indexes` → `repos` by `module_path`
  (no index filter), grouped per repo: identity, pinned version, used packages, used symbols (file:line).
- **D4 consumer linking (resolved):** for each usage, `NodesByFile(consumerIndex, usage.File)`,
  parse each node's `source_location` (`L<n>`), pick the node with the greatest line ≤ `usage.Line`
  (enclosing symbol); unresolved-marked when the file has no nodes or none precede.
- **Result typing:** lean-typed (mirror get_context) — typed provider/consumer/export/usage entries
  with explicit resolved/node ref; `limitations` always present.

## Key Trade-offs and Risks

- D4 best-effort: nearest-preceding-line approximates the enclosing definition; flagged
  unresolved when it can't, never errored.
- Touches `internal/store` (new cross-index query) — larger surface than get_context; covered by
  a dedicated store-level test.
- Surface drift 14 → 15 (`e2e_surface_test.go`).
- Provider repo that self-consumes appears in consumers (natural; any index with usage rows).

## Testing Strategy

TDD: store-level cross-index aggregation test (2 consumer indexes + 1 provider) first; then tool
provider+consumers; fuzzy best+candidates; provider-less; unknown→not-found; export→provider-node
link resolves; consumer usage→enclosing-node link resolves and unresolved-marked. `go test ./...` + `go vet ./...`.

## Spec Patches

Add a positive scenario under "Requirement: Code-graph linking for exports and usages" in
`specs/private-library-tools/spec.md`: a consumer usage links to the enclosing consumer node
(greatest definition line ≤ the usage line), complementing the existing unresolved-marker scenario.
