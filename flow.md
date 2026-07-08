# candle Flow

This document explains how candle works from setup to an agent answer.

## What candle does

candle is a private engineering knowledge layer exposed as an MCP stdio
server. It lets an AI agent answer questions across repositories by joining three
sources of information:

1. Code graphs from Graphify.
2. API contracts from OpenAPI and protobuf files.
3. Private Go library providers and consumers.

The important part is the linking step. candle does not only store parsed
contracts. It connects contract nodes back to code symbols, so an endpoint or RPC
can be traced to its handler, service calls, schemas, messages, and library usage.

## High-level flow

```text
Repository source code
        |
        v
Graphify produces graphify-out/graph.json
        |
        v
manifest.yaml points candle at each repo snapshot
        |
        v
candle index
        |
        +--> load code graph nodes and edges
        +--> parse OpenAPI specs
        +--> parse protobuf files
        +--> analyze Go modules and private imports
        +--> link contracts and libraries back to code symbols
        |
        v
SQLite database, grouped by index_id snapshots
        |
        v
candle serve
        |
        v
MCP client asks tools and reads resources
        |
        v
AI agent returns contract + implementation + dependency context
```

## Step 1: Produce a code graph

candle expects each indexed repository to already have a Graphify graph.
The graph contains code nodes, files, symbols, and edges such as calls.

Example input:

```text
/abs/inventory-service/graphify-out/graph.json
```

candle consumes this file. It does not parse source code itself for the main
code graph.

## Step 2: Describe repos in `manifest.yaml`

The manifest tells the indexer which repo snapshots to ingest and where their
inputs live.

```yaml
repos:
  - repo: org/inventory-service
    graph: /abs/inventory-service/graphify-out/graph.json
    commit: abc123
    branch: main
    openapi:
      - api/openapi.yaml
    proto:
      roots: [proto]
    go:
      modules: ["."]
      private_prefixes:
        - github.com/org/
```

Each manifest entry becomes one repo snapshot. The snapshot is stored under an
`index_id`, which keeps re-indexing idempotent and keeps different repos isolated.

## Step 3: Run the indexer

```bash
go run ./cmd/candle index --db intel.db --config manifest.yaml
```

During indexing, candle does this per repo:

1. Loads the Graphify `graph.json` through `internal/graph`.
2. Parses OpenAPI specs through `internal/openapi`.
3. Parses protobuf files through `internal/proto`.
4. Analyzes Go modules through `internal/godep`.
5. Links contracts and private libraries to code symbols through `internal/link`.
6. Writes everything into SQLite through `internal/store`.

Missing or malformed repo inputs are skipped with warnings so one bad repo does
not stop all indexing.

## Step 4: Store snapshots in SQLite

SQLite is the query store. The main unit is `index_id`, one indexed snapshot per
repo.

Stored data includes:

- Repo identity, branch, commit, and ingest time.
- Graphify nodes and edges.
- OpenAPI specs, HTTP operations, and schemas.
- Protobuf files, services, RPCs, messages, and enums.
- Go dependencies, private library exports, and private library usages.

candle does not merge every repo into one giant graph. Cross-repo answers
are computed by joining snapshots at query time.

## Step 5: Serve MCP over stdio

```bash
go run ./cmd/candle serve --db intel.db
```

The server opens the SQLite database, registers MCP tools and resources, then
speaks MCP over stdin/stdout. An MCP client such as Claude Desktop, Claude Code,
or another compatible agent starts this process and calls the tools.

Core tool groups:

- Repo and code graph: `list_repos`, `resolve_repo`, `query_repo`, `explain_symbol`, `get_file_context`.
- OpenAPI: `list_apis`, `find_endpoint`, `explain_endpoint`, `find_schema`.
- Protobuf: `find_rpc`, `explain_rpc`.
- Private libraries: `find_private_library`, `find_library_consumers`.

Resource URIs are commit-pinned where applicable, so lookups can refer to the
same snapshot later.

## How an endpoint question works

User asks:

```text
Which handler implements the reserve-product endpoint?
```

The agent typically calls:

```text
list_repos
  -> find_endpoint(repo=org/inventory-service, query="reserve product")
  -> explain_endpoint(repo=org/inventory-service, method=POST, path=/v1/reservations)
  -> explain_symbol(repo=org/inventory-service, symbol=<handler or operation match>)
```

candle answers by combining:

- OpenAPI operation details: method, path, operation ID, schemas, security, tags.
- Linking data: which code symbol implements the operation.
- Code graph traversal: handler callers and callees.

Result shape:

```text
POST /v1/reservations
  operationId: reserveProduct
  request: ReserveProductRequest
  response: Reservation
  implemented by: internal/http/reservation_handler.go:ReserveProduct
  calls: reservation_service_reserveproduct
```

## How an RPC question works

User asks:

```text
What does ReserveProduct RPC look like and who implements it?
```

The agent calls:

```text
find_rpc(repo=org/inventory-service, query="ReserveProduct")
  -> explain_rpc(repo=org/inventory-service, service=ReservationService, rpc=ReserveProduct)
```

candle answers by combining:

- Proto service and RPC metadata.
- Request and response message fields.
- Implementation symbols linked from the code graph.
- One-hop code calls from the implementation.

## How a private library question works

User asks:

```text
Who consumes our auth library and which symbols do they use?
```

The agent calls:

```text
find_private_library(repo=org/platform-libs, query="auth")
  -> find_library_consumers(repo=org/inventory-service, module="github.com/org/platform-libs/auth")
```

candle answers by combining:

- Provider-side exports: packages, functions, types, constructors, interfaces.
- Consumer-side imports: pinned version, used packages, and used symbols.
- Source locations where symbols are used.

## Why linking matters

Without linking, an agent can say that an OpenAPI operation exists. With linking,
it can say where that operation is implemented and what code path it enters.

Important edges include:

- `implemented_by`: contract operation or RPC -> handler/server symbol.
- `calls`: handler/server symbol -> service/repository/client symbol.
- `uses_schema`: HTTP operation -> OpenAPI schema.
- `uses_message`: RPC -> proto request or response message.
- `exports`: private library -> exported package or symbol.
- `uses_symbol`: consumer repo -> private exported symbol.
- `consumes`: consumer repo -> private library or contract surface.

These edges are what turn separate files into an engineering knowledge graph.

## Re-indexing flow

When a repo changes, update its Graphify output and/or contract files, then run
the index command again.

```bash
go run ./cmd/candle index --db intel.db --config manifest.yaml
```

Re-indexing replaces that repo snapshot instead of appending duplicate rows. This
makes repeated indexing safe during development.

## What is not yet implemented

The MVP supports OpenAPI, protobuf, and Go private modules. The following are not yet implemented:

- Automatic breaking-change detection.
- Full cross-repo RPC consumer aggregation.
- Generated-client analysis.
- API diff visualization.
- PR review automation.
- Dependency ecosystems beyond Go, such as npm, Python, Maven, Gradle, and Cargo.

## Mental model

Think of candle as a bridge:

```text
Contracts say what services promise.
Code graphs say where behavior lives.
Private library analysis says who shares code with whom.
candle links them so an agent can explain the real implementation path.
```
