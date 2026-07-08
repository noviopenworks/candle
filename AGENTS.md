# AGENTS.md

This file orients opencode (and any agent) when working in this repository.
It is a condensed companion to [docs/design.md](docs/design.md) (the source-of-truth
design spec) and [CONTRIBUTING.md](CONTRIBUTING.md). Claude Code users: the same
guidance lives in [CLAUDE.md](CLAUDE.md) — keep the two in sync.

## What this is

candle is a **private engineering knowledge layer** delivered as an **MCP server
(stdio)**. It combines three layers so an AI coding agent can reason about
service boundaries across many repos:

1. **Code graph** — produced by an external **Graphify** loader (symbols, calls,
   files). candle consumes `graph.json`; it does not parse source.
2. **API contract layer** — OpenAPI specs and protobuf, parsed, stored, and
   linked back to code.
3. **Private library layer** — internal Go modules, indexed from both the
   provider side (defines the lib) and the consumer side (imports it).

The core value is **not parsing** the contracts — it's **linking contracts back
to the code graph** so questions like *"which handler implements this OpenAPI
operation?"* or *"what breaks if I change this proto message?"* resolve across
repos.

## Build, test, lint

The toolchain (Go, golangci-lint, goreleaser, govulncheck) is pinned in
`mise.toml`; builds run through [Task](https://taskfile.dev).

```bash
go build ./...      # compile
go vet ./...        # static checks
go test ./...       # full suite (incl. the MCP end-to-end test)
task ci             # fmt-check, vet, lint, test, vuln  (the CI gate)
```

The `internal/mcp` e2e test builds the `candle` binary, indexes a fixture graph,
and drives the stdio server — so a green `go test ./...` means the whole
index → serve path works.

## Architecture

Indexer → SQLite store → MCP stdio server. One `index_id` per indexed repo
snapshot; contract/library tables hang off it.

Key packages (see [docs/architecture.md](docs/architecture.md) for the full map):

- `cmd/candle` — CLI entrypoint (`index`, `serve`).
- `internal/config` — manifest parsing (`candle.yaml` → `RepoConfig`).
- `internal/graph` — Graphify `graph.json` loader.
- `internal/openapi`, `internal/proto` — contract parsers.
- `internal/godep` — Go module / private-library parser (`go.mod`/`go.work`).
- `internal/link` — links contracts and exports to graph nodes (AST-precise when
  a source `root` is set, heuristic otherwise).
- `internal/ingest` — orchestrates the per-repo pipeline into the store.
- `internal/store` — SQLite schema, DDL, and all read/write queries.
- `internal/registry` — resolves repo names to `index_id`; enforces serve scope.
- `internal/mcp` — the 15 MCP tools and 5 resource URI schemes; SDK types stay
  in `internal/mcp/server.go`, tool methods are pure and SDK-free.

## Conventions

- **Module path:** `github.com/noviopenworks/candle`. **Binary:** `candle`
  (`cmd/candle`).
- **Default branch:** `main` (not `master`).
- Match surrounding code style, naming, and comment density. One logical change
  per commit; messages describe intent, not mechanics.
- **Docs:** follow [CONTRIBUTING.md](CONTRIBUTING.md) › Documentation conventions.
  Edit the minimum, preserve format, and verify every count/cross-reference
  (tool and test counts drift easily). No agentic rewrites or "vibecoding" the
  docs into inconsistency.

## Adding or changing a tool

1. Add the pure method to the relevant `internal/mcp/*_tools.go`, with a unit test.
2. Register it in `internal/mcp/server.go` (`ToolNames` + a `register*`).
3. Document it in [docs/tools.md](docs/tools.md) with example I/O.
4. Run `go test ./...` — the e2e test asserts the advertised tool surface.
