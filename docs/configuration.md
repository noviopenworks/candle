# Configuration: `manifest.yaml`

The manifest tells `candlegraph index` which repos to ingest and where to find
each repo's Graphify graph and contract files. It is passed with `--config`
(default `manifest.yaml`).

## Top-level shape

```yaml
repos:
  - repo: <org>/<name>      # required — repo identity
    graph: <path>           # required — Graphify graph.json
    commit: <sha>           # optional — pins snapshot identity
    branch: <name>          # optional
    openapi: [...]          # optional — OpenAPI spec paths
    proto:                  # optional — protobuf discovery
      roots: [...]
      files: [...]
    go:                     # optional — Go module indexing
      modules: [...]
      private_prefixes: [...]
```

`repos` is a list; each entry is one repo snapshot (`index_id`).

## Field reference

### `repo` (required)

The repo identity as `org/name`, e.g. `org/inventory-service`. Used as the
`repo` argument to every tool and as the `{org}/{name}` segments in resource URIs.

### `graph` (required)

Path to the repo's Graphify `graph.json`. Absolute paths are recommended so the
indexer resolves them regardless of working directory. If the file is missing or
malformed, the repo is **skipped** with a warning and the rest of the run
continues.

### `commit`, `branch` (optional)

Record the snapshot's VCS identity. `commit` is reflected in commit-pinned
resource URIs (`/commit/<sha>/`), making contract lookups reproducible.

### `openapi` (optional)

A list of OpenAPI spec paths (relative to the repo) to parse. Supports
OpenAPI 3 (`openapi.{yaml,yml,json}`) and Swagger 2 (`swagger.{yaml,json}`).

```yaml
openapi:
  - api/openapi.yaml
  - api/admin/openapi.yaml
```

### `proto` (optional)

Protobuf discovery. Use `roots` to scan directories, `files` to list specific
files, or both.

```yaml
proto:
  roots:
    - proto
    - api
  files:
    - internal/events/events.proto
```

### `go` (optional)

Go module indexing for the private-library layer.

- **`modules`** — module roots to analyze (each a directory containing a `go.mod`).
- **`private_prefixes`** — module-path prefixes that mark a dependency as
  *private/internal*. A dependency whose path starts with one of these is
  treated as a private library; everything else is a third-party dependency.

```yaml
go:
  modules:
    - .                       # the repo's own module
  private_prefixes:
    - github.com/noviopenworks/   # anything under this org is "private"
```

## Serve scope

`candlegraph serve` can use the same manifest shape as a serve-time scope. When
started with `--config <path>`, the MCP server exposes only snapshots matching the
listed repos. Without an explicit `--config`, serve looks for the default
`manifest.yaml` in the working directory; if it exists, that manifest scopes the
server. If no config is supplied or discovered, serve remains unscoped and exposes
all indexed snapshots in the SQLite store.

Each scope entry is resolved by `repo` plus `commit`:

- If `commit` is present, serve exposes that exact indexed snapshot.
- If `commit` is omitted, serve exposes the latest indexed snapshot for that repo.
- If a listed repo or pinned commit is not present in the store, serve prints a
  scope warning and continues with the snapshots it can resolve.

The `graph:` field is still required because the loader validates manifests for
indexing. For serve-only scope files, keep `graph:` pointed at the Graphify graph
used when the repo was indexed; serve itself resolves the scope from `repo` and
`commit` in the database.

Example: run an MCP instance that exposes only inventory and warehouse from a
larger multi-repo store:

```bash
candlegraph serve --db intel.db --config examples/serve-scope.yaml
```

## Worked examples

### HTTP service with OpenAPI

```yaml
repos:
  - repo: org/inventory-service
    graph: /abs/inventory/graphify-out/graph.json
    commit: abc123
    branch: main
    openapi:
      - api/openapi.yaml
```

### gRPC service with protobuf

```yaml
repos:
  - repo: org/reservation-service
    graph: /abs/reservation/graphify-out/graph.json
    branch: main
    proto:
      roots:
        - proto
```

### Go monorepo providing and consuming a private library

```yaml
repos:
  - repo: org/platform-libs           # provider: defines the library
    graph: /abs/platform-libs/graphify-out/graph.json
    go:
      modules: ["."]
      private_prefixes: ["github.com/org/"]

  - repo: org/inventory-service        # consumer: imports it
    graph: /abs/inventory/graphify-out/graph.json
    go:
      modules: ["."]
      private_prefixes: ["github.com/org/"]
```

After indexing both, `find_library_consumers` can report how
`org/inventory-service` consumes a module defined in `org/platform-libs`.

### All three layers in one repo

```yaml
repos:
  - repo: org/inventory-service
    graph: /abs/inventory/graphify-out/graph.json
    commit: abc123
    branch: main
    openapi:
      - api/openapi.yaml
    proto:
      roots: [proto]
    go:
      modules: ["."]
      private_prefixes: ["github.com/org/"]
```

## Validation and idempotency

- Each entry must have `repo` and `graph`.
- `repo` must be `org/name` (used to derive `Org()` and `Name()`).
- Re-running `index` **replaces** a repo's snapshot — safe to run repeatedly.

A ready-to-edit starter lives at [`examples/manifest.yaml`](../examples/manifest.yaml).
