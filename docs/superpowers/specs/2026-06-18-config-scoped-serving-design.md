---
comet_change: add-config-scoped-serving
role: technical-design
canonical_spec: openspec
archived-with: 2026-06-18-add-config-scoped-serving
status: final
---

# Config-scoped MCP serving — Technical Design

## Summary

Make `candle serve` scope its surface to a YAML config (the existing manifest). An instance
exposes only the `(repo, commit)` snapshots in its config and omits everything else in the store,
making resolution deterministic and version-correct and enabling multiple isolated instances. With
no config, `serve` behaves as today (serve-all) — additive and backward compatible.

## Architecture

```
candle serve --db DB [--config FILE]
   │  resolve config: explicit --config | discover ./manifest.yaml | none
   ▼
scope.Build(store, manifestEntries) -> {allowed []index_id, warnings}   (nil when no config)
   ▼
mcp.Serve(ctx, store, scope)
   └─ NewServer(store, scope) -> NewTools(store, scope) -> registry.NewScoped(store, allowed)
                                                            (scope==nil -> registry.New(store))
   ▼
tools resolve through reg (scoped) ; cross-repo aggregation filtered to `allowed` in Tools layer
```

## Components

### Config resolution (cmd/candle/main.go)
`serve` resolves which config file to use: if `--config` was set explicitly (`cmd.Flags().Changed("config")`)
use that path (error if it does not exist); else if the default `manifest.yaml` exists in the working
dir, use it; else no scope. Loads via the existing `config.Load`.

### Scope builder (new, e.g. internal/serve or internal/registry)
`BuildScope(s *store.Store, cfg *config.Config) (*Scope, []string, error)`:
- For each config entry `(org/name, commit)`: look up the matching `indexes` row
  (`repos.org/name` + `indexes.commit_sha` when `commit` set; else the latest by `ingested_at`).
- Collect matched `index_id`s into `Scope.Allowed` (a set). Missing entries → warning string, skipped.
- `Scope` is an opaque allow-set; `nil` means "no scope" (serve all).

### Scoped registry (internal/registry)
`NewScoped(s *store.Store, allowed map[int64]bool) *Registry` stores the allow-set; `New(s)` leaves it
nil. `List` filters rows to `allowed` (when non-nil); `Resolve` returns the single in-scope snapshot
per repo (deterministic); `Match` filters to `allowed`. No scope → current behavior.

### Tools wiring (internal/mcp)
`NewTools(s, scope)` / `NewServer(s, scope)` / `Serve(ctx, s, scope)` thread the scope to the
registry. Cross-repo aggregation (`explain_private_library`, `find_library_consumers` cross-repo)
filters returned consumers to repos whose `index_id ∈ allowed`, using the registry's allow-set;
`store.PrivateConsumersAcrossRepos` is unchanged.

## Key Decisions

| ID | Decision | Rationale |
|----|----------|-----------|
| D1 | Reuse the existing manifest as scope config | No new schema; manifest already carries `repo`+`commit`. |
| D2 | Build scope once at startup, inject into registry | Tools unchanged; registry is the single scope-enforcement point. |
| D3 | Scope-aware registry (allow-set) | `List`/`Resolve`/`Match` filter; deterministic Resolve under scope; nil = serve-all. |
| D4 | `commit` omitted ⇒ latest snapshot (by `indexed_at`) | Deterministic and convenient; explicit `commit` pins exactly. |
| D5 | Auto-discover `--config` (default manifest.yaml) from cwd; explicit overrides; absent ⇒ serve-all | Matches "config from where you use it"; preserves back-compat. |
| D6 | Missing configured `(repo, commit)` ⇒ warning, non-fatal | Serve what's present; don't fail startup. |
| D7 | Match by exact commit when set; branch informational | `commit_sha` is unique per snapshot; branch not needed to disambiguate. |
| D8 | Cross-repo scope enforced in the Tools layer | Keeps `store.PrivateConsumersAcrossRepos` generic; registry allow-set is the source of truth. |

## Data Flow

1. `serve` resolves config path (explicit / discovered / none).
2. If a config: `config.Load` → `BuildScope` → allow-set + warnings (printed to stderr).
3. `Serve(ctx, store, scope)` builds a scoped registry; tools resolve only in-scope snapshots.
4. Cross-repo aggregation drops out-of-scope consumers.
5. No config: scope nil → registry unscoped → identical to current behavior.

## Error Handling & Boundaries

- Explicit `--config` path missing → startup error (user asked for a specific file).
- Discovered default absent → no scope, serve-all (not an error).
- Configured `(repo, commit)` absent from store → warning, skipped, server still starts.
- Empty resulting scope (config present but nothing matched) → server starts but serves nothing;
  surfaced via warnings (a degenerate but valid instance).

## Testing Strategy

TDD, failing-test-first:
1. Scoped registry: multi-snapshot repo → `Resolve` deterministic to the allowed snapshot; `List`
   filters; nil scope unchanged.
2. `BuildScope`: pinned commit selects snapshot; omitted commit selects latest; missing entry warns.
3. `serve` discovery/precedence + e2e: config scoping a subset → `list_repos` returns only configured.
4. Cross-repo aggregation under scope only includes configured repos.
5. Backward-compat: no config → serve all.
Manual verification: instance scoped to service-inventory + warehouse-service over the 5-repo store;
`list_repos` shows only those two. Gates: `go test ./...`, `go vet ./...`, diff-scope check.

## Non-Goals

Per-call `commit`/`version` tool arguments; cross-instance federation; indexer/parser/storage/tool-IO
changes (beyond filtering); the all-numeric `commit:` YAML coercion bug (separate fix).
