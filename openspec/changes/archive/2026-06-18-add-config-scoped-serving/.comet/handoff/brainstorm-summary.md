# Brainstorm Summary

- Change: add-config-scoped-serving
- Date: 2026-06-18

## Confirmed Technical Approach

Thread a `Scope` value: `Serve(ctx, s, scope) → NewServer(s, scope) → NewTools(s, scope) →
registry.NewScoped(s, allowedIndexIDs)`; `scope == nil` → `registry.New(s)` (serve-all, unchanged).

- **Discovery:** `serve` reads `--config` (default `manifest.yaml`). File exists → build scope;
  absent → serve-all; explicit `--config <path>` overrides. ("Config from where you use it.")
- **Scope build:** load the manifest, match each `(org/name, commit)` against `indexes`/`repos`
  → allowed `index_id` set + warnings for missing entries (non-fatal).
- **Scoped registry:** `List`/`Resolve`/`Match` filter to allowed `index_id`s; `Resolve`
  deterministic (one snapshot per repo) under a scope.
- **commit-omitted = latest** snapshot of that repo by `ingested_at` (deterministic). With a
  `commit`, pin exactly that snapshot.
- **Match strictness = exact commit** when provided; `branch` is informational, not matched.
- **Cross-repo enforcement = Tools-layer filter:** `explain_private_library` /
  `find_library_consumers` drop consumers whose `index_id` is out of scope, using the scoped
  registry's allowed set; `store.PrivateConsumersAcrossRepos` stays generic (no signature change).

Worked example: an instance scoped to `VendSYSTEM/service-inventory` + `VendSYSTEM/warehouse-service`
(full repo index, both already indexed), omitting service-user / bff-service / platform-go.

## Key Trade-offs and Risks

- Auto-discovery: running `serve` in a dir containing `manifest.yaml` now scopes — intended but a
  behavior change for that case (documented). No config present anywhere → serve-all (back-compat).
- Tools-layer cross-repo filtering over-fetches then drops; acceptable at current scale, avoids
  changing the store query.

## Testing Strategy

TDD: scoped-registry filtering/determinism (multi-snapshot repo); config→allowed-index_id resolver
(pinned/latest/missing-warning); serve wiring + discovery precedence (e2e: list_repos returns only
configured); cross-repo aggregation respects scope; backward-compat (no config → serve all). Plus
the inventory+warehouse worked example as manual verification. `go test ./...` + `go vet ./...`.

## Spec Patches

Add a scenario under "Requirement: Config-scoped serving" in `specs/mcp-core/spec.md`: a config
entry without `commit` resolves to the repo's latest snapshot (by ingested_at), deterministically.
