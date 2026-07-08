# candle

**A private engineering knowledge layer, delivered as an [MCP](https://modelcontextprotocol.io) server.**

candle lets an AI coding agent reason about service boundaries across many
repositories. It combines three layers into one queryable graph:

1. **Code graph** — symbols, calls, and files, produced by an existing
   [Graphify](https://github.com/safishamsi/graphify) loader.
2. **API contract layer** — OpenAPI specs and protobuf, parsed and linked back to code.
3. **Private library layer** — internal Go modules, indexed from both the
   provider side (defines the lib) and the consumer side (imports it).

The core value is **not parsing** contracts — it's **linking contracts back to the
code graph** so questions like *"which handler implements this OpenAPI operation?"*
or *"what breaks if I change this proto message?"* resolve across repos.

```
   ┌─────────────┐     ┌──────────────┐     ┌──────────────────┐
   │  Code graph │     │ API contracts│     │ Private libraries│
   │ (Graphify)  │     │ OpenAPI/proto│     │   Go modules     │
   └──────┬──────┘     └──────┬───────┘     └────────┬─────────┘
          │                   │                      │
          └───────────────────┼──────────────────────┘
                              ▼
                    ┌───────────────────┐
                    │  SQLite index     │  one snapshot per repo (index_id)
                    └─────────┬─────────┘
                              ▼
                    ┌───────────────────┐
                    │  MCP stdio server │  15 tools · 5 resource schemes
                    └───────────────────┘
                              ▼
                         AI agent
```

## Installation

Any of these gives you a `candle` binary that speaks MCP over stdio.

**`go install`** (needs Go 1.26+):

```bash
go install github.com/noviopenworks/candle/cmd/candle@latest
```

It lands in `$(go env GOBIN)` (or `$(go env GOPATH)/bin`) — make sure that's on
your `PATH`.

**Prebuilt release binary** — download the archive for your OS/arch from the
[releases page](https://github.com/noviopenworks/candle/releases), extract,
and put `candle` on your `PATH`:

```bash
tar -xzf candle_<version>_<os>_<arch>.tar.gz
./candle --help
```

**From source (for development)** — the repo pins its toolchain with
[mise](https://mise.jdx.dev) and drives builds with
[Task](https://taskfile.dev):

```bash
git clone https://github.com/noviopenworks/candle
cd candle
mise install   # Go + linter + release tools, versions from mise.toml
task install   # go install ./cmd/candle
```

Run `task -l` for all developer tasks (build, test, lint, vuln, coverage,
release). Full walkthrough in **[docs/getting-started.md](docs/getting-started.md)**.

## Quick start

```bash
# 1. Build
go build ./...

# 2. Describe your repos in a manifest (see docs/configuration.md)
cp examples/candle.yaml candle.yaml

# 3. Index the repos into a SQLite snapshot store
go run ./cmd/candle index --db intel.db --config candle.yaml
# → indexed=2 skipped=0

# 4. Run the MCP stdio server
go run ./cmd/candle serve --db intel.db
```

Then point an MCP client (Claude Desktop, Claude Code, any MCP-compatible agent)
at the `serve` command. See **[docs/getting-started.md](docs/getting-started.md)**.

Agents typically start with `get_context`: call it with a repo for a catalog of
what candle knows, or with a repo plus a topic for focused Context7-style retrieval.

To run isolated MCP instances over the same store, pass a manifest subset to
`serve --config`. For example,
[`examples/serve-scope.yaml`](examples/serve-scope.yaml) exposes only
`VendSYSTEM/service-inventory` and `VendSYSTEM/warehouse-service`:

```bash
go run ./cmd/candle serve --db intel.db --config examples/serve-scope.yaml
```

Use a different scope file per MCP client; omit `--config` only when the client
should use the default `candle.yaml` scope or, if no config is present, see all
indexed snapshots.

## Documentation

| Doc | What's in it |
|-----|--------------|
| [Getting started](docs/getting-started.md) | Install, index, serve, connect a client |
| [How it works](flow.md) | End-to-end narrative from setup to an agent's answer |
| [Concepts](docs/concepts.md) | The three layers, the graph model, cross-repo joins, commit pinning |
| [Configuration](docs/configuration.md) | Full `candle.yaml` reference |
| [Tools reference](docs/tools.md) | All 15 MCP tools with arguments and example I/O |
| [Resources reference](docs/resources.md) | The 5 URI schemes for commit-pinned lookups |
| [Examples](docs/examples.md) | End-to-end walkthroughs (find a handler, impact analysis, consumers) |
| [Architecture](docs/architecture.md) | Internal packages, data flow, storage layout |
| [Contributing](CONTRIBUTING.md) | Build/test gate, conventions, doc style |

## Status

MVP. Contract parsers cover three ecosystems: **OpenAPI**, **protobuf**, and
**Go modules**. Automatic breaking-change detection and additional dependency
ecosystems (npm, pyproject, Maven, Cargo) are not yet implemented — see
[docs/concepts.md](docs/concepts.md#whats-not-yet-implemented).

The complete design spec lives in **[docs/design.md](docs/design.md)**.

## License

MIT — see [LICENSE](LICENSE). © 2026 noviopenworks.
