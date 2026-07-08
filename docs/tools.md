# MCP tools reference

The server advertises **16 tools**, in this registration order:

`list_repos` · `resolve_repo` · `get_context` · `query_repo` · `explain_symbol` ·
`get_file_context` · `call_path` · `list_apis` · `find_endpoint` · `explain_endpoint` ·
`find_schema` · `find_rpc` · `explain_rpc` · `find_private_library` ·
`find_library_consumers` · `explain_private_library`

Every tool returns a single JSON text payload. Tools that take a `repo` resolve
it internally; an unknown repo/symbol comes back as a **tool-level error
result** (graceful `not found: <reason>`, e.g. `not found: repo "org/x" not
indexed`) rather than a protocol error.

> **JSON casing.** Example payloads below mirror the actual Go structs. Some
> structs carry `json:` tags (snake_case, e.g. `module_path`); others serialize
> with Go field names (e.g. `OperationID`, `SourceFile`). Each example uses the
> real casing.

---

## Repo & code-graph tools

### `list_repos`

List all indexed repository snapshots with node counts. No arguments.

**Response** — array of repo snapshots:

```json
[
  {"IndexID": 1, "Repo": "org/inventory-service", "Branch": "main", "Commit": "abc123", "IngestedAt": "2026-06-17T10:00:00Z", "NodeCount": 412},
  {"IndexID": 2, "Repo": "org/warehouse-service", "Branch": "", "Commit": "", "IngestedAt": "2026-06-17T10:00:01Z", "NodeCount": 188}
]
```

### `resolve_repo`

Resolve a repo query to a snapshot: exact match first, else fuzzy candidates.

| Arg | Type | Description |
|-----|------|-------------|
| `query` | string | repo identity (`org/name`) or fuzzy substring |

**Response** — `{ "best": <snapshot or null>, "candidates": [<snapshots>] }`:

```json
{
  "best": {"IndexID": 1, "Repo": "org/inventory-service", "Branch": "main", "Commit": "abc123", "IngestedAt": "2026-06-17T10:00:00Z", "NodeCount": 412},
  "candidates": []
}
```

For a fuzzy query like `"invent"`, `best` is null and `candidates` lists matches.

### `get_context`

Context7-style retrieval entry point — the recommended **first** call. With only
`repo`, it returns a catalog of what candle knows about that repo (code graph,
OpenAPI, protobuf, private libraries) with counts and the precise follow-up tools for
each surface. With `topic`, it searches code symbols, HTTP endpoints, schemas, RPCs,
proto messages, and private libraries in that repo, returning code matches with one-hop
callers/callees.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity (`org/name`) |
| `topic` | string | optional symbol / endpoint / RPC / schema / library topic |
| `mode` | string | optional: `overview`, `code`, `api`, `proto`, `library`, `all` (`overview` returns the catalog only and suppresses topic matches; unknown/empty ⇒ `all`) |
| `depth` | number | optional; v1 supports one-hop code context |
| `include_resources` | boolean | include exact resource URI hints |

**Overview request:**

```json
{"repo": "org/inventory-service"}
```

**Topic request:**

```json
{"repo": "org/inventory-service", "topic": "ReserveProduct", "include_resources": true}
```

**Response** — a typed `repo` summary, grouped `capabilities`, `matches` (in topic mode),
`resources` URI hints, `suggested_next_calls`, and explicit `limitations`. Follow the
`suggested_next_calls` into precise tools (`explain_symbol`, `explain_rpc`,
`explain_endpoint`, `find_private_library`) once a surface is identified.

### `query_repo`

Structural node lookup in a repo by symbol label.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity (`org/name`) |
| `name` | string | symbol label to look up |

**Response** — array of `NodeRow`:

```json
[
  {"IndexID": 1, "NodeID": "reservation_service_reserveproduct", "Label": "ReserveProduct", "FileType": "code", "SourceFile": "internal/reservation/service.go", "SourceLocation": ""}
]
```

### `explain_symbol`

Explain a symbol: its node plus callers and callees. `symbol` may be a node id
**or** a label.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |
| `symbol` | string | node id or label to explain |

**Response** — `{ Node, Callers, Callees }`:

```json
{
  "Node": {"IndexID": 1, "NodeID": "reservation_service_reserveproduct", "Label": "ReserveProduct", "FileType": "code", "SourceFile": "internal/reservation/service.go", "SourceLocation": ""},
  "Callers": [
    {"Source": "http_reservation_reserveproduct", "Target": "reservation_service_reserveproduct", "Relation": "calls"}
  ],
  "Callees": [
    {"Source": "reservation_service_reserveproduct", "Target": "inventory_repo_decrement", "Relation": "calls"}
  ]
}
```

### `get_file_context`

List the symbols defined in a given source file.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |
| `file` | string | source file path |

**Response** — array of `NodeRow` defined in that file.

---

### `call_path`

Multi-hop call traversal from a symbol, returned as a tree. `explain_symbol`
is one-hop; `call_path` walks the call graph up to `depth` hops so a chain like
handler → service → repository → client is one call.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |
| `symbol` | string | node id or label to traverse from (first label match wins) |
| `depth` | int | max hops (default 1, max 5) |
| `direction` | string | `callees` (default) · `callers` · `both` |

Cycles are cut by a per-path visited set, so a diamond or back-edge does not
loop. Each hop carries the node and the edge that reached it (`via`, nil for the
root).

```json
{
  "node": {"NodeID": "http_reserveproduct", "Label": "ReserveProduct", "SourceFile": "internal/http/handler.go"},
  "children": [
    {
      "node": {"NodeID": "reservation_service_reserveproduct", "Label": "ReserveProduct", "SourceFile": "internal/reservation/service.go"},
      "via": {"Source": "http_reserveproduct", "Target": "reservation_service_reserveproduct", "Relation": "calls"},
      "children": []
    }
  ]
}
```

---

## OpenAPI tools

### `list_apis`

List the API contracts indexed for a repo. Output is **kind-discriminated** —
the same shape serves OpenAPI and protobuf, distinguished by `kind`.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |

**Response** — array of `APIInfo`:

```json
[
  {"kind": "openapi", "name": "Inventory API", "version": "1.2.0", "path": "api/openapi.yaml"},
  {"kind": "proto",   "name": "reservation.v1", "version": "",      "path": "proto/reservation/v1/reservation.proto"}
]
```

### `find_endpoint`

Find HTTP operations by **lexical** match on path / method / operationId / summary.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |
| `query` | string | NL / path / method / operationId |

**Response** — array of matching `HTTPOperation` (see `explain_endpoint` shape).

### `explain_endpoint`

Explain an HTTP endpoint's full contract.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |
| `method` | string | HTTP method (`GET`, `POST`, …) |
| `path` | string | endpoint path template |

**Response** — `HTTPOperation`:

```json
{
  "Method": "POST",
  "Path": "/v1/reservations",
  "OperationID": "reserveProduct",
  "Summary": "Reserve a product for an order",
  "RequestSchema": "ReserveProductRequest",
  "ResponseSchema": "Reservation",
  "Security": ["bearerAuth"],
  "Tags": ["reservations"],
  "SpecPath": "api/openapi.yaml"
}
```

### `find_schema`

Find OpenAPI component schemas by name substring.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |
| `query` | string | schema name substring |

**Response** — array of schema descriptors (`kind`, `name`, `spec_path`).

---

## Protobuf tools

### `find_rpc`

Find gRPC RPCs by lexical match, optionally filtered by stream kind.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |
| `query` | string | NL / service / rpc / full name |
| `stream_kind` | string | optional: `unary` \| `server_stream` \| `client_stream` \| `bidi` |

**Response** — array of matching RPCs (`ProtoRPCResult`).

### `explain_rpc`

Explain a gRPC RPC: proto facts, message fields, same-repo implementation, and
one-hop calls.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |
| `service` | string | gRPC service name |
| `rpc` | string | RPC method name |

**Response** — `RPCExplanation`:

```json
{
  "rpc": {"Name": "ReserveProduct", "Service": "ReservationService", "ProtoPath": "proto/reservation/v1/reservation.proto"},
  "request_message_fields": [
    {"name": "product_id", "type": "string", "number": 1, "label": ""},
    {"name": "quantity",   "type": "int32",  "number": 2, "label": ""}
  ],
  "response_message_fields": [
    {"name": "reservation_id", "type": "string", "number": 1, "label": ""}
  ],
  "implemented_by": [
    {"NodeID": "reservation_server_reserveproduct", "SourceFile": "internal/grpc/server.go"}
  ],
  "calls": [
    {"Source": "reservation_server_reserveproduct", "Target": "reservation_service_reserveproduct", "Relation": "calls"}
  ],
  "consumed_by": ["org/warehouse-service", "org/order-service"]
}
```

> `consumed_by` is a **heuristic**: it lists repos whose code graph contains a
> node labelled like the RPC (a gRPC client-call signal), excluding the provider
> and any repo that defines the RPC. candle does not index gRPC client calls, so
> a label match is the available cross-repo signal.

---

## Private-library tools

### `find_private_library`

Find internal Go libraries by name, module path, package path, or purpose.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |
| `query` | string | name / module path / purpose |

**Response** — array of private-library descriptors.

### `find_library_consumers`

Show how a repo consumes a private Go module: the version pinned and the
symbols actually used.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity |
| `module` | string | module path of the private library |

**Response** — `LibraryConsumers`:

```json
{
  "module_path": "github.com/org/platform-libs/auth",
  "version": "v1.4.0",
  "used_packages": ["auth", "auth/jwt"],
  "used_symbols": [
    {"ModulePath": "github.com/org/platform-libs/auth", "Version": "v1.4.0", "PackagePath": "auth", "Symbol": "ValidateToken", "File": "internal/mw/auth.go", "Line": 22}
  ],
  "consumed_across_repos": ""
}
```

> `consumed_across_repos` is empty in this release (cross-repo aggregation is not
> yet implemented), like `explain_rpc`'s `consumed_by`. For cross-repo library
> consumers, use `explain_private_library`.

### `explain_private_library`

Explain an internal Go library from both sides: the provider definition (exports with code-graph node links, packages, doc synopsis) and **cross-repo consumers** — every indexed repo that uses the library, with its pinned version and used symbols (each best-effort linked to the enclosing consumer code-graph node).

| Arg | Type | Description |
|-----|------|-------------|
| `query` | string | library name, module path, doc synopsis, or purpose (fuzzy) |

**Request:**

```json
{"query": "auth"}
```

**Response:** `provider` (module path, exports with `node`/`resolved`, packages), `consumers` (per repo: version, used packages, usages with `node`/`resolved`), `candidates` when ambiguous, and `limitations`. Unlike `find_library_consumers` (single repo, empty `consumed_across_repos`), this aggregates consumers across all indexed repos.

---

## Error behavior

- **Unknown repo / symbol / endpoint** → a tool-level error result with text
  `not found` (the call "succeeds" at the protocol level but is marked `IsError`).
- **Malformed input** → a protocol error.

This lets agents probe for things that may not exist without aborting a session.

See [examples.md](examples.md) for tool chains, and [resources.md](resources.md)
for commit-pinned URI lookups of the same artifacts.
