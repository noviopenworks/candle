# Concepts

candlegraph answers cross-repo questions by joining three layers of information
about your services into one graph, then linking the contract layers back to the
code layer. This document explains the model.

## The three layers

### Layer 1: Code graph (Graphify)

candlegraph does **not** parse source code. It consumes the `graph.json` produced
by a [Graphify](https://github.com/safishamsi/graphify) loader, which already
extracted:

- **nodes** — symbols (functions, types), with `id`, `label`, `file_type`,
  `source_file`, and optional `source_location`.
- **edges** — relationships such as `calls`, with `relation`, `confidence`, and `weight`.
- **hyperedges** — group relationships (optional).

A minimal graph fixture:

```json
{
  "nodes": [
    {"id": "http_reservation_reserveproduct", "label": "ReserveProduct", "file_type": "code", "source_file": "internal/http/reservation_handler.go", "source_location": "L10"},
    {"id": "reservation_service_reserveproduct", "label": "ReserveProduct", "file_type": "code", "source_file": "internal/reservation/service.go"}
  ],
  "edges": [
    {"source": "http_reservation_reserveproduct", "target": "reservation_service_reserveproduct", "relation": "calls", "confidence": "EXTRACTED", "confidence_score": 1.0, "weight": 1.0}
  ],
  "hyperedges": []
}
```

### Layer 2: API contract layer

candlegraph parses contract files and normalizes them into graph nodes that
**link to code symbols**:

- **OpenAPI** (`openapi.{yaml,yml,json}`, `swagger.{yaml,json}`, `api/**`) →
  specs, HTTP operations, schemas.
- **Protobuf** (`proto/**/*.proto`, `api/**/*.proto`) → files, packages,
  services, RPCs, messages, enums.

### Layer 3: Private library layer

Internal Go modules are indexed from **both sides**:

- **Provider** — the repo that defines the library exports its packages/symbols.
- **Consumer** — every repo that imports it records the version pinned and the
  specific symbols actually used.

A private library is classified by **module-path prefix** (see
[configuration.md](configuration.md#go)).

## The linking edges are the point

Parsing each contract is table stakes. The value is the edges that bridge a
contract node to a code symbol or to another repo:

| Edge | Bridges |
|------|---------|
| `implemented_by` | an OpenAPI operation / proto RPC → its handler or server symbol |
| `calls` | handler → domain service → repository, within the code graph |
| `consumes` | a consumer repo → a proto RPC or private library it depends on |
| `uses_symbol` | a consumer → the specific exported symbol it imports |

An OpenAPI operation links to its handler symbol, which `calls` a domain service
symbol; a proto RPC links to its gRPC server impl and (eventually) to the
consumer repos that `consume` it.

## index_id: one snapshot per repo

Every indexed repo snapshot gets an **`index_id`**. All contract and library
rows hang off it. This is the unit of:

- **Idempotent re-indexing** — re-running `index` replaces a repo's `index_id`
  rows rather than appending.
- **Isolation** — two repos never share node ids by accident; lookups are scoped
  to a resolved `index_id`.

## Cross-repo relations are query-time joins

candlegraph does **not** merge all repos into one giant graph. Each repo stays
its own snapshot. Cross-repo questions ("who consumes this library?") are
answered by **joining at query time** across snapshots. This keeps each repo's
graph reproducible and lets you re-index one repo without rebuilding others.

```
   org/inventory-service (index_id=1)        org/warehouse-service (index_id=2)
   ┌──────────────────────────┐              ┌──────────────────────────┐
   │ proto RPC ReserveProduct  │   consumes   │ client calls Reserve...   │
   │   implemented_by handler  │◀─────────────│                           │
   └──────────────────────────┘   (join)      └──────────────────────────┘
```

## Commit pinning

Resources are addressable by commit (`/commit/<sha>/` in the URI), so a contract
lookup is reproducible against a specific snapshot. The `commit` and `branch`
fields in the manifest record the identity that pinning reflects. See
[resources.md](resources.md).

## Repo resolution

Tools take a `repo` argument as `org/name`. The `resolve_repo` tool turns a
fuzzy query into a snapshot: it tries an **exact** identity match first, then
returns fuzzy **candidates**. Most tools resolve the repo internally and return
a graceful not-found error for unknown repos rather than a protocol error.

## What's deferred

These are explicitly **out of MVP scope**:

- Automatic breaking-change detection and API diffing
- Cross-repo `consumed_by` aggregation for RPCs (the field exists but is deferred)
- Generated-client analysis, SDK generation, PR-review automation
- Dependency ecosystems beyond Go: npm, pyproject, Maven/Gradle, Cargo

See [design.md](design.md) for the full rationale and roadmap.
