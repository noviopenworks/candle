# Comet Design Handoff

- Change: rename-to-candlegraph
- Phase: design
- Mode: compact
- Context hash: 7a6de8832cbfceee25f22685d0c97dc974a151cb1e0c7aeefb33b7e1f86b011d

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/rename-to-candlegraph/proposal.md

- Source: openspec/changes/rename-to-candlegraph/proposal.md
- Lines: 1-26
- SHA256: f136516c4b9b4affb2f8a53586aa06527cc8bb9da537d34cbf7999e5f26f84aa

```md
## Why

The Go module was published under a placeholder path (`github.com/candlegraph/intel-mcp`) and an `intel-mcp` binary sub-name that do not match the canonical repository, `github.com/noviopenworks/candlegraph`. Aligning the module path and binary/command name with the real repo is required before the module can be imported or installed by its true path.

## What Changes

- **BREAKING** (import path): Go module path `github.com/candlegraph/intel-mcp` → `github.com/noviopenworks/candlegraph`. Every internal import is updated accordingly.
- Binary / command name `intel-mcp` → `candlegraph` (cobra `Use`, MCP server `Name`, e2e-built binary).
- Command directory `cmd/intel-mcp/` → `cmd/candlegraph/`.
- Historical implementation plans under `docs/superpowers/plans/` updated so their `cmd/intel-mcp` / `intel-mcp` references stay accurate.
- No behavior, API surface, storage schema, or dependency changes.

## Capabilities

### New Capabilities
- `module-identity`: Declares the canonical Go module path and the single binary/command name the project ships under, plus the rule that no stale prior name remains in tracked source.

### Modified Capabilities
<!-- None. This is a pure identity/path correction; no existing capability's requirements change. -->

## Impact

- **Code**: `go.mod`; all `.go` files (imports + literal name strings); `cmd/intel-mcp/ → cmd/candlegraph/`; `internal/mcp/server.go` (`Name`), `cmd/candlegraph/main.go` (cobra `Use`), `internal/mcp/e2e_test.go` (built binary name).
- **Docs**: 4 plan files under `docs/superpowers/plans/`.
- **Out of scope**: generated `graphify-out/` knowledge-graph artifacts (regenerated separately; still carry the old path until rebuilt); archived openspec changes (never referenced either name); external consumers (former consumer repo no longer exists).
- **Dependencies**: none added or removed.
```

## openspec/changes/rename-to-candlegraph/design.md

- Source: openspec/changes/rename-to-candlegraph/design.md
- Lines: 1-40
- SHA256: 53679a3ad37ec697dcc3ac3b8f6cded0139798008260d8e4beed333a3fcffa83

```md
## Context

The module shipped under a placeholder identity (`github.com/candlegraph/intel-mcp`, binary `intel-mcp`). The canonical repository is `github.com/noviopenworks/candlegraph`. This is a mechanical identity correction with no behavioral change. It was implemented and verified in the working tree before this change was opened; the design below records the approach taken.

## Approach

A two-pass, order-sensitive find-and-replace across tracked files (excluding generated artifacts and archived records):

1. **Module path first** — replace `github.com/candlegraph/intel-mcp` → `github.com/noviopenworks/candlegraph` everywhere. Doing this first means the resulting path no longer contains the substring `intel-mcp`, so the second pass cannot corrupt it.
2. **Bare name second** — replace remaining literal `intel-mcp` → `candlegraph` (cobra `Use`, MCP server `Name`, e2e binary path/comment).
3. **Directory move** — `git mv cmd/intel-mcp cmd/candlegraph` to preserve history.

```
   pass 1: module path          pass 2: bare name
   ┌──────────────────────┐     ┌──────────────────────┐
   │ candlegraph/intel-mcp│     │ intel-mcp (literal)  │
   │        │             │     │        │             │
   │        ▼             │     │        ▼             │
   │ noviopenworks/       │     │   candlegraph        │
   │   candlegraph        │     │                      │
   └──────────────────────┘     └──────────────────────┘
   ordering guarantees pass 2 cannot touch the new module path
```

## Decisions and Rationale

- **Order matters**: module-path pass precedes bare-name pass; reversing it would split the module path mid-string. (See diagram.)
- **Scope exclusions**: generated `graphify-out/` graph artifacts are regenerated, not hand-edited; archived openspec changes are point-in-time records and never referenced either name, so both are excluded.
- **Directory rename via `git mv`**: preserves file history rather than delete+add.
- **No spec-behavior change**: the only spec captured is the non-functional `module-identity` capability; no existing capability's requirements move.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` (11 packages) — all pass.
- `grep -r intel-mcp` over tracked files excluding `graphify-out/` — zero hits.

## Risks / Trade-offs

- **Import-path break for downstream consumers**: the only known consumer repo (`/home/mg/vend-ai/`) no longer exists, so there is no live breakage. Any future consumer must import the new path.
- **Stale knowledge graph**: `graphify-out/graph.json` still carries old-path nodes until regenerated — deferred, out of scope.
```

## openspec/changes/rename-to-candlegraph/tasks.md

- Source: openspec/changes/rename-to-candlegraph/tasks.md
- Lines: 1-26
- SHA256: 55bb7c6762ec572a89a308cb05fb365f4cc88e454a505c8ccfbeddcd30ba92ce

```md
# Tasks — rename-to-candlegraph

> Implementation was completed in the working tree before this change was opened (retroactive capture). Tasks are checked off in the build phase against the existing diff.

## 1. Module path

- [ ] 1.1 Update `go.mod` module directive to `github.com/noviopenworks/candlegraph`
- [ ] 1.2 Update all internal `.go` import paths to the new module path (pass 1, before bare-name pass)

## 2. Binary / command name

- [ ] 2.1 `git mv cmd/intel-mcp cmd/candlegraph`
- [ ] 2.2 Update cobra root `Use` to `candlegraph` in `cmd/candlegraph/main.go`
- [ ] 2.3 Update MCP server `Name` to `candlegraph` in `internal/mcp/server.go`
- [ ] 2.4 Update e2e-built binary name and comment in `internal/mcp/e2e_test.go`

## 3. Docs

- [ ] 3.1 Update `intel-mcp` / `cmd/intel-mcp` references in the 4 `docs/superpowers/plans/` files

## 4. Verification

- [ ] 4.1 `go build ./...` passes
- [ ] 4.2 `go vet ./...` passes
- [ ] 4.3 `go test ./...` passes (all packages)
- [ ] 4.4 `grep -r intel-mcp` over tracked files (excluding `graphify-out/`) returns zero hits
```

## openspec/changes/rename-to-candlegraph/specs/module-identity/spec.md

- Source: openspec/changes/rename-to-candlegraph/specs/module-identity/spec.md
- Lines: 1-35
- SHA256: a780cd25c05c7a54d8a23d3b0c53f445ab2d71a4984e0073b0baa430c4e92d1f

```md
# module-identity

## ADDED Requirements

### Requirement: Canonical Go module path

The project SHALL declare its Go module path as `github.com/noviopenworks/candlegraph`, matching the canonical repository, and all internal imports SHALL resolve under that path.

#### Scenario: go.mod declares the canonical path

- **WHEN** `go.mod` is inspected
- **THEN** the module directive reads `module github.com/noviopenworks/candlegraph`

#### Scenario: internal imports resolve under the canonical path

- **WHEN** the module is built with `go build ./...`
- **THEN** every internal import path begins with `github.com/noviopenworks/candlegraph/` and the build succeeds

### Requirement: Single canonical binary name

The project SHALL ship under a single binary/command name, `candlegraph`, used consistently for the command entrypoint, the cobra root command, and the MCP server identity.

#### Scenario: command and server identify as candlegraph

- **WHEN** the command directory, cobra root `Use`, and MCP server `Name` are inspected
- **THEN** each is `candlegraph` and the entrypoint lives at `cmd/candlegraph/`

### Requirement: No stale prior name in tracked source

Tracked source, configuration, and documentation (excluding generated `graphify-out/` artifacts and archived openspec changes) SHALL contain no references to the prior `intel-mcp` name or the prior `github.com/candlegraph/intel-mcp` module path.

#### Scenario: grep finds no prior-name references

- **WHEN** `grep -r` for `intel-mcp` is run over tracked files, excluding `graphify-out/`
- **THEN** zero matches are returned
```

