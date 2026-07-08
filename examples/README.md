# Examples

Runnable starting points for candle.

## `candle.yaml`

A fully-commented sample manifest covering all four shapes:

- HTTP service with OpenAPI
- gRPC service with protobuf
- Go provider of a private library
- consumer that imports the library and has its own OpenAPI

Copy it to the repo root and edit the paths:

```bash
cp examples/candle.yaml candle.yaml
# edit the `graph:` paths to point at your real graphify-out/graph.json files
candle index --db intel.db --config candle.yaml
candle serve --db intel.db
```

## What you need first

Each repo needs a Graphify `graph.json`. candle consumes that output — it
does not extract code itself. Produce one per repo with Graphify, then point the
manifest's `graph:` field at the resulting `graphify-out/graph.json`.

## Tool-chain walkthroughs

For copy-pasteable agent tool calls (find an endpoint, explain an RPC, impact
analysis, library consumers), see **[../docs/examples.md](../docs/examples.md)**.

## Minimal graph fixture

The smallest valid Graphify graph candle will index — a handler that calls
a service:

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
