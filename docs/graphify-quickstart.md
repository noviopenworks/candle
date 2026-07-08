# Graphify quickstart

candle does not extract code itself — it consumes a **Graphify `graph.json`** per
repo. This page is the verified contract for that file: the fields candle reads,
which are required, and how to validate your output. Produce the file with
Graphify, or hand-author one to match the schema below.

> The schema documented here is verified against candle's loader
> ([`internal/graph/graph.go`](../internal/graph/graph.go),
> [`internal/graph/loader.go`](../internal/graph/loader.go)). Graphify's own CLI
> invocation may differ by version; consult Graphify's docs for the command, then
> point candle at the resulting `graphify-out/graph.json`.

## 1. What candle needs

One `graph.json` per repo snapshot, referenced by the `graph:` field in
[`candle.yaml`](configuration.md):

```yaml
repos:
  - repo: org/inventory-service
    graph: /abs/path/inventory/graphify-out/graph.json
    commit: abc123
    branch: main
```

The graph describes **nodes** (symbols/files), **edges** (relationships, chiefly
`calls`), and optionally **hyperedges** (n-ary relationships). candle stores them
verbatim under one `index_id` and joins contracts/libraries back to the nodes.

## 2. The `graph.json` schema

Top level:

```json
{
  "nodes":      [ ... ],
  "edges":      [ ... ],
  "hyperedges": [ ... ]
}
```

All three arrays are optional, but a graph with no nodes is useless to candle.

### Node

| Field | Required | Type | Notes |
|-------|----------|------|-------|
| `id` | yes | string | unique within the repo snapshot; the join key |
| `label` | no | string | symbol name; used by `query_repo`, `explain_symbol`, `call_path`, and the RPC/HTTP linkers (name match) |
| `file_type` | no | string | e.g. `code`, `doc` |
| `source_file` | no | string | path to the defining file |
| `source_location` | no | string | `L<n>` form (e.g. `L10`); parsed for usage-to-node linking |
| `source_url`, `captured_at`, `author`, `contributor` | no | string | provenance; stored, not queried |

### Edge

| Field | Required | Type | Notes |
|-------|----------|------|-------|
| `source` | yes | string | node id of the caller/origin |
| `target` | yes | string | node id of the callee/destination |
| `relation` | yes | string | **`calls`** is the relation candle traverses (one-hop and `call_path`); other relations are stored but not walked |
| `confidence` | no | string | e.g. `EXTRACTED` |
| `confidence_score` | no | float | |
| `weight` | no | float | |
| `source_file` | no | string | |

### Hyperedge

| Field | Required | Type | Notes |
|-------|----------|------|-------|
| `id` | yes | string | |
| `nodes` | no | string[] | member node ids |
| `label`, `relation`, `confidence`, `confidence_score`, `source_file` | no | | stored; not currently queried |

## 3. Minimal valid graph

The smallest graph candle will index — an HTTP handler that calls a service
(this is also [`examples/README.md`](../examples/README.md#minimal-graph-fixture)):

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

## 4. Producing a graph

- **With Graphify** — run Graphify against the repo, then point `graph:` at the
  resulting `graphify-out/graph.json`. Ensure each symbol of interest has a
  stable `id`, a human-readable `label` (the symbol name), and a `source_file`.
  Call edges should use `relation: "calls"`.
- **By hand or with another extractor** — author a JSON file matching the schema
  above. This is practical for small fixtures and for evaluating candle without
  Graphify. Node `id`s only need to be unique within the snapshot; `label`
  should match the symbol name so the contract linkers can find impls.

## 5. Validate

Index with `--verbose` and watch stderr for skips:

```bash
candle index --db intel.db --config candle.yaml --verbose
```

Malformed entries — a node without `id`, an edge missing `source`/`target`/
`relation`, or a hyperedge without `id` — are **skipped with a warning**, not
fatal. The run prints `indexed=N skipped=M`; investigate any non-zero `skipped`.

Then confirm the graph is queryable:

```bash
candle serve --db intel.db
# in your MCP client: list_repos, then query_repo for a label you indexed
```

If `query_repo` returns the node and `explain_symbol` returns its callers/
callees, the graph is correctly loaded.
