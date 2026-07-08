## Why

A candle store can hold many repos, and the same repo at multiple commits/versions
(`indexes` is keyed by `UNIQUE(repo_id, commit_sha)`). But `registry.Resolve(repo)` matches
on `org/name` only and returns the **first** snapshot it finds (`ORDER BY org, name`, no tiebreak
among a repo's snapshots) — so with multiple versions indexed, every repo-scoped tool silently
answers against an **arbitrary, non-deterministic** snapshot, and there is no way to target a
specific version. Teams that want to run **isolated MCP instances** — each scoped to a specific
set of repos at specific versions — cannot: an instance exposes whatever happens to be in the DB.

## What Changes

- `candle serve` SHALL **scope the served surface to a YAML config** (the existing manifest).
  An instance exposes **only** the `(repo, commit)` pairs declared in its config and **omits**
  every other repo/version present in the store.
- The config **pins each repo to a version** (the manifest's existing `commit:`), making
  `resolve_repo` / repo resolution **deterministic and version-correct**.
- The config is loaded from the existing `--config` flag and/or discovered from the working
  location, so each instance is scoped by the config "where it is used". Multiple instances
  (distinct configs, same or separate DBs) stay isolated.
- All repo-scoped tools — `list_repos`, `resolve_repo`, `get_context`, `explain_symbol`,
  `find_endpoint`, `find_rpc`, `find_private_library`, `find_library_consumers`,
  `explain_private_library` (cross-repo aggregation) — operate **only on the configured set**.
- A configured `(repo, commit)` not present in the store is **skipped with a surfaced warning**,
  not a crash.

Backward compatibility: when no config is provided (or discovered), `serve` behaves as today —
all indexed snapshots are served — so this is additive/opt-in.

## Capabilities

### New Capabilities
<!-- None; this extends mcp-core. -->

### Modified Capabilities
- `mcp-core`: MODIFY "Repo listing and resolution tools" so listing/resolution honor a serve-time
  config scope and resolve deterministically to a pinned snapshot; ADD a "Config-scoped serving"
  requirement describing how `serve` filters the surface to the configured `(repo, commit)` set.

## Impact

- **Code:** `cmd/candle/main.go` (wire `--config` into `serve` + discovery), `internal/registry`
  (config-aware, deterministic resolution + scoped `List`/`Match`), `internal/config` (serve-scope
  usage of the existing manifest schema; possibly a thin scope accessor), and the cross-repo
  aggregation entry (`internal/store` `PrivateConsumersAcrossRepos`) to respect the configured set.
- **Reused:** the existing manifest schema (`repo`, `commit`, `branch`) — no new config format.
- **Docs:** `docs/configuration.md`, `docs/getting-started.md`/`README.md` (serve scoping, multi-instance).
- **No change** to the indexer, parsers, storage schema, or tool I/O shapes (beyond filtering).
- **Out of scope:** per-call `commit`/`version` tool arguments; cross-instance federation; the
  all-numeric-`commit:` YAML coercion bug (separate fix).
