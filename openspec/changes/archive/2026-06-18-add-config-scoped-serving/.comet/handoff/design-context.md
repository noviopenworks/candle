# Comet Design Handoff

- Change: add-config-scoped-serving
- Phase: design
- Mode: compact
- Context hash: 223ab9043f21ee10b2e967d473275c57b8d16e0bdf2b0fafed093c74f53e4133

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/add-config-scoped-serving/proposal.md

- Source: openspec/changes/add-config-scoped-serving/proposal.md
- Lines: 1-50
- SHA256: 007d0aa085d505994b215f566d70c467113ff522230b6d9637e4a1231541c7da

```md
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
```

## openspec/changes/add-config-scoped-serving/design.md

- Source: openspec/changes/add-config-scoped-serving/design.md
- Lines: 1-70
- SHA256: 1531173c4e031f607bc5a01a7cde8aec9aba183da922e47004abfa620d245ec1

```md
## Context

`cmd/candle/main.go` already declares `--config` as a persistent flag (default
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
filename TBD in design, e.g. `manifest.yaml`/`candle.yaml` in cwd). No config found → serve all
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
```

## openspec/changes/add-config-scoped-serving/tasks.md

- Source: openspec/changes/add-config-scoped-serving/tasks.md
- Lines: 1-50
- SHA256: 00b8d72398cf9f897e67b418f9bbc47b5eab66294d654adb4240b5ace4f356f6

```md
# Tasks: add-config-scoped-serving

> TDD-oriented. The worked example throughout is an instance scoped to
> `VendSYSTEM/service-inventory` + `VendSYSTEM/warehouse-service` (omitting the other
> indexed repos), per the user's target use case.

## 1. Scope model in the registry (test-first)

- [ ] 1.1 Add a failing registry test: given a store with multiple repos and multiple snapshots of one repo, a scoped registry over a chosen `(repo, commit)` set lists/resolves only those; resolution is deterministic to the pinned snapshot
- [ ] 1.2 Run the test and confirm it fails
- [ ] 1.3 Make `registry` scope-aware: build a scope from allowed `index_id`s; `List`/`Resolve`/`Match` filter to the scope; `nil` scope = serve-all (unchanged). Resolve deterministically to the single in-scope snapshot per repo
- [ ] 1.4 Run the test and confirm it passes

## 2. Resolve config → allowed snapshots (test-first)

- [ ] 2.1 Add a failing test: given the manifest entries (`repo` + optional `commit`) and the store, compute the allowed `index_id` set; pinned commit selects that snapshot; missing `(repo, commit)` yields a warning, not an error; commit-omitted resolves per the design-decided default
- [ ] 2.2 Run and confirm it fails
- [ ] 2.3 Implement the config→scope resolver (matches configured `(org/name, commit)` against `indexes`/`repos`), returning allowed `index_id`s + warnings
- [ ] 2.4 Run and confirm it passes

## 3. Wire `serve` to the scope config

- [ ] 3.1 Add a failing test (or e2e surface assertion) that `serve` with a config scoping to a subset exposes only those repos via `tools/list`-reachable tools (`list_repos` returns only configured repos)
- [ ] 3.2 Wire `--config` into `serve` in `cmd/candle/main.go`; on startup build the scope and construct a scoped registry/Tools; no config → serve-all
- [ ] 3.3 Implement working-location discovery + precedence (explicit `--config` wins; else discover from cwd; else serve-all) per design
- [ ] 3.4 Run and confirm pass

## 4. Constrain cross-repo aggregation to the scope

- [ ] 4.1 Add a failing test: `explain_private_library` / `find_library_consumers` cross-repo aggregation under a scope only aggregates configured repos
- [ ] 4.2 Run and confirm it fails
- [ ] 4.3 Constrain `store.PrivateConsumersAcrossRepos` (or the Tools-layer caller) to the allowed `index_id`s per the design decision
- [ ] 4.4 Run and confirm it passes

## 5. Worked example + manual verification: inventory + warehouse

- [ ] 5.1 Provide an example serve config scoping to exactly `VendSYSTEM/service-inventory` and `VendSYSTEM/warehouse-service` (e.g. `examples/serve-scope.yaml`)
- [ ] 5.2 Manually verify against a multi-repo store: with that config, `list_repos` returns only service-inventory + warehouse-service; service-user / bff-service / platform-go are omitted from every tool
- [ ] 5.3 Record the manual verification result in the verification report

## 6. Documentation

- [ ] 6.1 Update `docs/configuration.md`: serve-time scope config, `commit` pinning semantics, discovery/precedence, missing-snapshot warning
- [ ] 6.2 Update `docs/getting-started.md` / `README.md`: running multiple isolated, config-scoped MCP instances; the inventory+warehouse example

## 7. Final verification

- [ ] 7.1 Run `go test ./...` and confirm pass
- [ ] 7.2 Run `go vet ./...` and confirm pass
- [ ] 7.3 Inspect `git diff` and confirm scope matches the plan
```

## openspec/changes/add-config-scoped-serving/specs/mcp-core/spec.md

- Source: openspec/changes/add-config-scoped-serving/specs/mcp-core/spec.md
- Lines: 1-66
- SHA256: ebfce77391a761d28971c9e58ab9cfc55b235c294e285b6d7f08eb0fc1c442de

```md
# mcp-core Specification

## MODIFIED Requirements

### Requirement: Repo listing and resolution tools
The system SHALL provide `list_repos` returning indexed repos with snapshot metadata, and
`resolve_repo` returning the best matching repo for a fuzzy query. When the server is started
with a scope config, listing and resolution SHALL be limited to the configured `(repo, commit)`
snapshots, and resolution SHALL be deterministic: a repo configured with a pinned commit SHALL
always resolve to that snapshot, never to another snapshot of the same repo.

#### Scenario: list_repos returns indexed repos
- **WHEN** `list_repos` is called with at least one indexed repo
- **THEN** it returns each repo with `repo`, `branch`, `commit`, `indexed_at`, and `node_count`

#### Scenario: resolve_repo fuzzy matches
- **WHEN** `resolve_repo` is called with a partial or approximate name
- **THEN** it returns the best matching `org/repo` identity (with candidates when ambiguous)

#### Scenario: Resolution is deterministic when multiple snapshots exist
- **WHEN** the store holds the same repo at more than one commit and the server is started with a
  scope config that pins that repo to one commit
- **THEN** `resolve_repo` and every repo-scoped tool resolve that repo to the pinned snapshot,
  and the other snapshots of that repo are not returned

#### Scenario: list_repos honors the configured scope
- **WHEN** the server is started with a scope config listing a subset of the store's repos
- **THEN** `list_repos` returns only the configured repos at their configured commits, omitting
  all other repos and other-commit snapshots

## ADDED Requirements

### Requirement: Config-scoped serving
`candle serve` SHALL accept a scope config (the manifest schema, via the existing `--config`
flag or discovery from the working location). When a scope config is present, the server SHALL
expose only the `(repo, commit)` snapshots declared in it and SHALL omit every other repo and
snapshot in the store from all tools and resources. When no scope config is present, the server
SHALL serve all indexed snapshots (current behavior), making the feature opt-in and backward
compatible.

#### Scenario: Unconfigured repos are omitted everywhere
- **WHEN** the store holds repos A, B, and C and the server is started with a config listing only
  A and B
- **THEN** repo C does not appear in `list_repos`, does not resolve via `resolve_repo`, and every
  repo-scoped tool treats C as not found

#### Scenario: Cross-repo aggregation respects the scope
- **WHEN** `explain_private_library` (or `find_library_consumers` cross-repo aggregation) runs under
  a scope config
- **THEN** it aggregates consumers only among the configured repos, ignoring snapshots outside the
  configured set

#### Scenario: Configured snapshot missing from the store is skipped, not fatal
- **WHEN** the scope config pins a `(repo, commit)` that is not present in the store
- **THEN** the server starts and serves the snapshots that are present, surfacing a warning for the
  missing entry rather than failing

#### Scenario: No config serves everything (backward compatible)
- **WHEN** `serve` is started without a scope config
- **THEN** all indexed snapshots are served exactly as before this change

#### Scenario: Config entry without a commit resolves to the latest snapshot
- **WHEN** a scope config entry names a repo but omits `commit`, and the store holds one or more
  snapshots of that repo
- **THEN** the server resolves that repo to its latest snapshot (by `indexed_at`) deterministically,
  and that single snapshot is the one served for the repo
```

