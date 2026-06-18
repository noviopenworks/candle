## Context

`cmd/candlegraph/main.go` already declares `--config` as a persistent flag (default
`manifest.yaml`) but `serve` ignores it — only `index` uses it. `registry.Resolve(repo)` lists all
snapshots (`indexes JOIN repos`, `ORDER BY org, name`) and returns the first `org/name` match, so
multiple snapshots of one repo resolve arbitrarily. Every tool resolves through `t.reg.Resolve`,
and cross-repo aggregation (`store.PrivateConsumersAcrossRepos`) scans all indexes unfiltered.

The manifest schema (`internal/config`) already carries `repo`, `commit`, and `branch` per entry —
exactly the `(repo, commit)` pins this change needs. So the scope config is the **existing
manifest**, reused at serve time; no new config format.

## Goals / Non-Goals

**Goals**
- `serve` filters the served surface to the configured `(repo, commit)` set; deterministic,
  version-correct resolution; multi-instance isolation by config; backward compatible (no config →
  serve everything).

**Non-Goals**
- Per-call `commit`/`version` tool arguments (instance-level config pinning is the mechanism).
- Cross-instance federation/merging.
- Indexer, parser, storage-schema, or tool-I/O changes (beyond filtering).
- The all-numeric `commit:` YAML coercion bug (separate fix).

## Decisions

### D1: Reuse the existing manifest as the serve-scope config
No new schema. `serve` loads the same manifest `index` uses; the set of `(repo, commit)` entries is
the scope. Entries without `commit` are resolved per D4.

### D2: Scope is built once at serve startup and injected into the registry
`serve` resolves the config to a concrete set of allowed `index_id`s (matching configured
`(org/name, commit)` against the store) and constructs a **scoped registry** over that set. Tools
are unchanged — they keep calling `t.reg.Resolve` / `t.reg.List`; the registry enforces the scope.

### D3: Registry becomes scope-aware
`registry.List`/`Resolve`/`Match` filter to the allowed `index_id`s. With a scope, `Resolve` is
deterministic (one snapshot per repo). Without a scope (`nil`), behavior is unchanged (serve all).
The cross-repo aggregation entry must also be constrained to the allowed set (pass the allowed
`index_id`s down, or filter results in the Tools layer).

### D4: `commit` semantics in the scope config
A configured entry with a `commit` pins exactly that snapshot. The design phase must confirm the
default when `commit` is omitted — candidate: resolve to the latest snapshot of that repo (by
`ingested_at`), documented explicitly. Strictness (exact commit vs commit+branch) decided in design.

### D5: Discovery and precedence
`serve` uses `--config` when given; otherwise discovers a config from the working location (default
filename TBD in design, e.g. `manifest.yaml`/`candlegraph.yaml` in cwd). No config found → serve all
(backward compatible). Exact precedence finalized in brainstorming.

### D6: Missing configured snapshot is non-fatal
A configured `(repo, commit)` absent from the store is skipped with a surfaced warning; the server
still serves the present entries.

## Risks / Trade-offs

- **Cross-repo aggregation path:** `store.PrivateConsumersAcrossRepos` currently joins across all
  indexes; it must respect the scope. Cleanest is to pass allowed `index_id`s; alternative is
  Tools-layer post-filtering. Design picks one.
- **Scope vs DB duplication:** a per-instance separate DB and a shared-DB-with-filter both work
  under this model; the config is the single source of scope either way.
- **Backward compatibility:** default-serve-all must be preserved when no config is present.

## Open Questions (for design/brainstorming)
- Default filename + discovery precedence for the working-location config (D5).
- `commit`-omitted semantics: latest vs require-pin (D4).
- Match strictness: exact commit vs commit+branch.
- Cross-repo scope enforcement: push allowed `index_id`s into the store query vs filter in Tools.
