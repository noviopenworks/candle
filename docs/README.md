# candle documentation

Start here, then follow the path that matches what you're doing.

## Reading paths

**"I want the 5-minute picture of how it works."**
→ [How it works](../flow.md) — end-to-end narrative from setup to an answer.

**"I want to run it."**
→ [Getting started](getting-started.md) → [Configuration](configuration.md)

**"I want to query it from an agent."**
→ [Tools reference](tools.md) → [Resources reference](resources.md) → [Examples](examples.md)

**"I want to understand or extend it."**
→ [Concepts](concepts.md) → [Architecture](architecture.md) → [Contributing](../CONTRIBUTING.md)

**"I want the full spec."**
→ [design.md](design.md) — the source-of-truth design document (all tool I/O,
full DDL, edge catalog, end-to-end walkthrough).

## Index

| Document | Summary |
|----------|---------|
| [getting-started.md](getting-started.md) | Install prerequisites, build, index repos, run the server, connect Claude. |
| [flow.md](../flow.md) | End-to-end narrative: how a repo becomes an indexed snapshot and how an agent query is answered. |
| [concepts.md](concepts.md) | Why three layers, the graph node/edge model, `index_id`, query-time cross-repo joins, commit pinning, what's deferred. |
| [configuration.md](configuration.md) | Every `manifest.yaml` field with examples for OpenAPI, protobuf, and Go. |
| [tools.md](tools.md) | All 13 MCP tools: arguments, behavior, example request and response JSON. |
| [resources.md](resources.md) | The `repo://`, `graph://`, `openapi://`, `proto://`, `lib://` URI schemes. |
| [examples.md](examples.md) | Concrete, copy-pasteable walkthroughs that chain tools together. |
| [architecture.md](architecture.md) | Package layout, ingestion pipeline, storage, how linking works. |
| [design.md](design.md) | The complete design spec. |

## Conventions used in these docs

- **Repo identity** is always `org/name` (e.g. `org/inventory-service`).
- **Example JSON** reflects the actual Go structs the server marshals. Fields
  without a `json:` tag serialize with their Go name (e.g. `OperationID`,
  `SourceFile`); fields with a tag use the tagged name (e.g. `module_path`).
  Each tool's doc notes which casing applies.
- Shell snippets assume you built the `candle` binary or use `go run ./cmd/candle`.
