# Getting started

This guide takes you from a clone to an agent querying your services.

## Prerequisites

- **Go 1.26+** (the module targets `go 1.26.5`) — or **[mise](https://mise.jdx.dev)**,
  which pins Go and the dev tools from the repo's `mise.toml` (`mise install`).
- **A Graphify code graph** per repo you want to index. candle consumes
  Graphify's `graph.json` output — it does not extract code itself. See the
  [Graphify quickstart](graphify-quickstart.md) for the schema and how to
  produce or hand-author a compliant file.
- **An MCP client** to talk to the server (Claude Desktop, Claude Code, or any
  MCP-compatible agent runner). The server speaks MCP over **stdio**.

## 1. Install or build

Install a ready-to-run binary with `go install` (needs Go 1.26+):

```bash
go install github.com/noviopenworks/candle/cmd/candle@latest
candle --help
```

Or grab a prebuilt archive for your OS/arch from the
[releases page](https://github.com/noviopenworks/candle/releases), extract
it, and put `candle` on your `PATH`.

To work from a clone instead:

```bash
git clone https://github.com/noviopenworks/candle
cd candle
go build ./...                       # build all packages
go build -o candle ./cmd/candle   # or a single binary
./candle --help
```

If you use [mise](https://mise.jdx.dev), `mise install` pins the toolchain and
`task install` runs the `go install` above. `task -l` lists every developer task
(build, test, lint, vuln, coverage, release).

The CLI has two subcommands and two persistent flags:

```
candle [--db intel.db] [--config candle.yaml] <command>

Commands:
  index   Ingest repo graphs from the manifest into the store
  serve   Run the MCP stdio server

Flags:
  --db       SQLite database path   (default "intel.db")
  --config   repo manifest path     (default "candle.yaml")
  --verbose  emit structured debug logs on stderr (default off)
```

All logging goes to **stderr**, so it never corrupts the MCP stdio transport
(which speaks JSON-RPC on stdout). With `--verbose`, `index` logs per-repo
progress and `serve` logs startup and repo-resolution misses; without it, only
warnings and errors are emitted.

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
cp examples/candle.yaml candle.yaml
```

Full field reference (protobuf roots, Go modules, private prefixes) is in
**[configuration.md](configuration.md)**.

## 3. Index

Indexing reads each repo's graph + contracts and writes one **snapshot**
(`index_id`) into the SQLite store. It is **idempotent** — re-running replaces a
repo's snapshot rather than duplicating it.

```bash
go run ./cmd/candle index --db intel.db --config candle.yaml
```

Output:

```
indexed=2 skipped=0
```

Repos whose graph file is missing or malformed are skipped with a warning on
stderr (the run still succeeds for the others).

## 4. Serve

```bash
go run ./cmd/candle serve --db intel.db
```

The process now speaks MCP on stdin/stdout. It blocks until the client
disconnects or the context is cancelled.

### Running multiple scoped instances

Use `--config` with a manifest subset to run isolated MCP instances over the same
SQLite store. For example, [`examples/serve-scope.yaml`](../examples/serve-scope.yaml)
exposes only `VendSYSTEM/service-inventory` and `VendSYSTEM/warehouse-service`:

```bash
go run ./cmd/candle serve --db intel.db --config examples/serve-scope.yaml
```

Start another `serve` process with a different scope file to give another MCP
client a different repo view. If `--config` is omitted, serve discovers
`candle.yaml` in the working directory when present; if no config is found, it
serves every indexed snapshot.

## 5. Connect a client

candle speaks MCP over stdio, so any MCP-compatible client works. Ready-to-copy
configs live in [`examples/clients/`](../examples/README.md#client-configs).

### Claude Desktop

Add to `claude_desktop_config.json` (see
[`examples/clients/claude-desktop.json`](../examples/clients/claude-desktop.json)):

```json
{
  "mcpServers": {
    "candle": {
      "command": "/absolute/path/to/candle",
      "args": ["serve", "--db", "/absolute/path/to/intel.db"]
    }
  }
}
```

### Claude Code

```bash
claude mcp add candle -- /absolute/path/to/candle serve --db /absolute/path/to/intel.db
```

### opencode

Add to `opencode.json` or `opencode.jsonc` at the project root, or to
`~/.config/opencode/opencode.json` for global use (see
[`examples/clients/opencode.json`](../examples/clients/opencode.json)):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "candle": {
      "type": "local",
      "command": ["/absolute/path/to/candle", "serve", "--db", "/absolute/path/to/intel.db"],
      "enabled": true
    }
  }
}
```

To serve a scoped subset of repos, add `--config` and a path to a
[`candle.yaml`](configuration.md#serve-scope) scope file to the `args`/`command`
list.

### Any MCP client

Launch `candle serve --db <path>` as a stdio MCP server. The server
advertises 17 tools and 5 resource templates (see [tools.md](tools.md) and
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
