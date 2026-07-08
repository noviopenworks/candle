# Architecture

How candle is put together internally, for contributors and the curious.

## Package layout

```
cmd/candle/        CLI entrypoint (cobra): `index` and `serve`
internal/
  config/               candle.yaml loading + validation (viper)
  graph/                Graphify graph.json loader (tolerant schema)
  store/                SQLite storage: schema, ingestion, query helpers
  ingest/               orchestrates per-repo indexing into the store
  registry/             resolves repo identity (org/name) → index_id snapshot
  openapi/              OpenAPI / Swagger parser → normalized bundles
  proto/                protobuf parser (protocompile) → normalized bundles
  godep/                Go module analysis: exports + usages, private classify
  link/                 links contract nodes ↔ code symbols (the bridge)
  mcp/                  MCP stdio server: tools, resources, SDK wiring
  version/              build version string
```

The dependency direction is one-way: `cmd` → `ingest`/`mcp` →
`store`/`config`/parsers. SDK types are confined to `internal/mcp/server.go`;
the tool methods themselves are pure and SDK-free, which keeps them unit-testable.

## Two entrypoints

```
            ┌───────────────────────── index ─────────────────────────┐
manifest →  config.Load  →  ingest.Run( store, cfg )                   │
                              │  per repo:                             │
                              │   graph.Load(graph.json)               │
                              │   openapi.Parse(specs)                 │
                              │   proto.Parse(files/roots)             │
                              │   godep.Analyze(modules)               │
                              │   link.* (contract ↔ code)             │
                              │   store.Upsert… (one index_id)         │
                              └────────────────────────────────────────┘

            ┌───────────────────────── serve ─────────────────────────┐
            store.Open(db) → mcp.Serve(ctx, store) → MCP over stdio    │
            └──────────────────────────────────────────────────────────┘
```

### `index` pipeline

1. **`config.Load`** parses `candle.yaml` into `Config{ Repos []RepoConfig }`.
2. **`ingest.Run`** iterates repos. For each it:
   - loads the Graphify graph (`graph.Load`),
   - parses any OpenAPI specs, proto files, and Go modules,
   - runs **`link`** to attach contract nodes to code symbols,
   - writes everything under a fresh `index_id` (idempotent replace).
3. Returns a report: `indexed`, `skipped`, and `warnings`. Missing/malformed
   graphs are skipped, not fatal.

### `serve` runtime

`mcp.NewServer(store)` registers all 15 tools and 5 resource templates against
the MCP Go SDK, then `Run`s on a `StdioTransport`. Tool handlers call pure
`Tools` methods on the store and marshal the result to a JSON text payload.

## Storage model

SQLite. The unit of indexing is a **snapshot** identified by `index_id`. Core
tables hang off it:

- `repos` / snapshot rows — identity, branch, commit, node count
- code graph — `nodes`, `edges` (from Graphify)
- OpenAPI — `api_specs`, `http_operations`, `api_schemas`
- protobuf — `proto_files`, `proto_services`, `proto_rpcs`, `proto_messages`
- dependencies — `dependencies` (with `is_private` flag and `ecosystem`),
  `private_library_exports`, `private_library_usages`

Full DDL is in [design.md](design.md#storage-extension).

### Why snapshots, not one merged graph

Each repo is indexed independently under its own `index_id`. Cross-repo
questions are answered by **joining at query time** across snapshots rather than
merging input. This keeps single-repo re-indexing cheap and reproducible. See
[concepts.md → cross-repo](concepts.md#cross-repo-relations-are-query-time-joins).

## The linking layer

`internal/link` is where the value lives. It matches:

- OpenAPI operations / proto RPCs → their implementing code symbols
  (`implemented_by`),
- private-library exports → the code nodes that define them,
- (consumer side) imported symbols → the exports they use.

Matching uses signals like service-registration presence and signature
similarity, with confidence tiers (HIGH/MEDIUM/LOW). Because linking is its own
package, both the proto and Go layers reuse one matcher.

## Graphify schema tolerance

`internal/graph` reads `graph.json` defensively: unknown fields are ignored and
missing optional fields (e.g. `source_location`, `hyperedges`) are tolerated, so
candle keeps working as Graphify's output evolves.

## Testing

Pure tool methods and parsers are unit-tested; `internal/mcp` has an
end-to-end test that builds the `candle` binary, indexes a fixture graph,
and drives the stdio server. Run the suite with:

```bash
go test ./...
```

See [CONTRIBUTING.md](../CONTRIBUTING.md) for the development workflow.
