# candlegraph

**A private engineering knowledge layer, delivered as an [MCP](https://modelcontextprotocol.io) server.**

candlegraph lets an AI coding agent reason about service boundaries across many
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
                    │  MCP stdio server │  14 tools · 5 resource schemes
                    └───────────────────┘
                              ▼
                         AI agent
```

## Quick start

```bash
# 1. Build
go build ./...

# 2. Describe your repos in a manifest (see docs/configuration.md)
cp examples/manifest.yaml manifest.yaml

# 3. Index the repos into a SQLite snapshot store
go run ./cmd/candlegraph index --db intel.db --config manifest.yaml
# → indexed=2 skipped=0

# 4. Run the MCP stdio server
go run ./cmd/candlegraph serve --db intel.db
```

Then point an MCP client (Claude Desktop, Claude Code, any MCP-compatible agent)
at the `serve` command. See **[docs/getting-started.md](docs/getting-started.md)**.

Agents typically start with `get_context`: call it with a repo for a catalog of
what candlegraph knows, or with a repo plus a topic for focused Context7-style retrieval.

## Documentation

| Doc | What's in it |
|-----|--------------|
| [Getting started](docs/getting-started.md) | Build, index, serve, connect a client |
| [Concepts](docs/concepts.md) | The three layers, the graph model, cross-repo joins, commit pinning |
| [Configuration](docs/configuration.md) | Full `manifest.yaml` reference |
| [Tools reference](docs/tools.md) | All 14 MCP tools with arguments and example I/O |
| [Resources reference](docs/resources.md) | The 5 URI schemes for commit-pinned lookups |
| [Examples](docs/examples.md) | End-to-end walkthroughs (find a handler, impact analysis, consumers) |
| [Architecture](docs/architecture.md) | Internal packages, data flow, storage layout |
| [Contributing](CONTRIBUTING.md) | Build/test, the OpenSpec + Comet workflow |

## Status

MVP. Contract parsers cover three ecosystems: **OpenAPI**, **protobuf**, and
**Go modules**. Cross-repo `consumed_by` aggregation, automatic breaking-change
detection, and additional dependency ecosystems (npm, pyproject, Maven, Cargo)
are deferred — see [docs/concepts.md](docs/concepts.md#whats-deferred).

The complete design spec lives in **[docs/design.md](docs/design.md)**.

## License

See the repository for license terms.
