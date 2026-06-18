# Getting started

This guide takes you from a clone to an agent querying your services.

## Prerequisites

- **Go 1.26+** (the module targets `go 1.26.3`).
- **A Graphify code graph** per repo you want to index. candlegraph consumes
  Graphify's `graph.json` output — it does not extract code itself. See
  [Concepts → Code graph](concepts.md#layer-1-code-graph-graphify) for how to
  produce one.
- **An MCP client** to talk to the server (Claude Desktop, Claude Code, or any
  MCP-compatible agent runner). The server speaks MCP over **stdio**.

## 1. Build

```bash
git clone https://github.com/noviopenworks/candlegraph
cd candlegraph
go build ./...
```

Build a binary if you prefer:

```bash
go build -o candlegraph ./cmd/candlegraph
./candlegraph --help
```

The CLI has two subcommands and two persistent flags:

```
candlegraph [--db intel.db] [--config manifest.yaml] <command>

Commands:
  index   Ingest repo graphs from the manifest into the store
  serve   Run the MCP stdio server

Flags:
  --db      SQLite database path   (default "intel.db")
  --config  repo manifest path     (default "manifest.yaml")
```

## 2. Write a manifest

The manifest lists each repo snapshot and where to find its Graphify graph and
contract files. Minimal example:

```yaml
repos:
  - repo: org/inventory-service
    graph: /abs/path/inventory/graphify-out/graph.json
    commit: abc123
    branch: main
    openapi:
      - api/openapi.yaml
  - repo: org/warehouse-service
    graph: /abs/path/warehouse/graphify-out/graph.json
```

Copy the starter and edit it:

```bash
cp examples/manifest.yaml manifest.yaml
```

Full field reference (protobuf roots, Go modules, private prefixes) is in
**[configuration.md](configuration.md)**.

## 3. Index

Indexing reads each repo's graph + contracts and writes one **snapshot**
(`index_id`) into the SQLite store. It is **idempotent** — re-running replaces a
repo's snapshot rather than duplicating it.

```bash
go run ./cmd/candlegraph index --db intel.db --config manifest.yaml
```

Output:

```
indexed=2 skipped=0
```

Repos whose graph file is missing or malformed are skipped with a warning on
stderr (the run still succeeds for the others).

## 4. Serve

```bash
go run ./cmd/candlegraph serve --db intel.db
```

The process now speaks MCP on stdin/stdout. It blocks until the client
disconnects or the context is cancelled.

### Running multiple scoped instances

Use `--config` with a manifest subset to run isolated MCP instances over the same
SQLite store. For example, [`examples/serve-scope.yaml`](../examples/serve-scope.yaml)
exposes only `VendSYSTEM/service-inventory` and `VendSYSTEM/warehouse-service`:

```bash
go run ./cmd/candlegraph serve --db intel.db --config examples/serve-scope.yaml
```

Start another `serve` process with a different scope file to give another MCP
client a different repo view. If `--config` is omitted, serve discovers
`manifest.yaml` in the working directory when present; if no config is found, it
serves every indexed snapshot.

## 5. Connect a client

### Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "candlegraph": {
      "command": "/absolute/path/to/candlegraph",
      "args": ["serve", "--db", "/absolute/path/to/intel.db"]
    }
  }
}
```

### Claude Code

```bash
claude mcp add candlegraph -- /absolute/path/to/candlegraph serve --db /absolute/path/to/intel.db
```

### Any MCP client

Launch `candlegraph serve --db <path>` as a stdio MCP server. The server
advertises 13 tools and 5 resource templates (see [tools.md](tools.md) and
[resources.md](resources.md)).

## 6. Try a query

Ask your agent something like:

> "List the indexed repos, then find the endpoint that reserves a product in
> `org/inventory-service` and show which handler implements it."

Under the hood that chains `list_repos` → `find_endpoint` → `explain_endpoint`.
See **[examples.md](examples.md)** for the exact tool calls and responses.

## Updating an index

Re-run `index` whenever a repo's graph or contracts change. Because indexing is
idempotent per repo, you can re-index a single repo by keeping only it in the
manifest, or re-index everything — existing snapshots are replaced cleanly.
