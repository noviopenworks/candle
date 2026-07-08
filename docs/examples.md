# Examples

End-to-end walkthroughs that chain tools together. Each shows the **question**,
the **tool calls** an agent makes, and the **shape of the answer**. JSON is
trimmed for readability; see [tools.md](tools.md) for full payloads.

Assume two repos are indexed: `org/inventory-service` (HTTP + gRPC + Go) and
`org/warehouse-service` (consumer).

---

## 0. Start with `get_context`

The recommended entry point. Call it with just a repo to discover what candle
knows, then with a topic for focused, Context7-style retrieval.

**Step 1 — repo catalog** (`get_context`):

```json
// request
{"repo": "org/inventory-service"}
// response (trimmed) — capability counts + the precise tools per surface
{"repo": {"repo": "org/inventory-service", "commit": "abc123", "node_count": 412},
 "capabilities": {"code_graph": {"count": 412, "tools": ["query_repo", "explain_symbol", "..."]},
                  "openapi": {"count": 1}, "protobuf": {"count": 1}, "private_libraries": {"count": 1}},
 "suggested_next_calls": [{"tool": "get_context", "args": {"topic": "<symbol …>"}}],
 "limitations": ["OpenAPI/HTTP handler linking is name-based; path→handler binding is coarse …"]}
```

**Step 2 — focused topic** (`get_context` with `topic`):

```json
// request
{"repo": "org/inventory-service", "topic": "ReserveProduct", "include_resources": true}
// response (trimmed) — matches across surfaces + resource URIs + follow-up hints
{"matches": {"code_symbols": [{"node": {"Label": "ReserveProduct"}, "callees": [/* one hop */]}],
             "schemas": [{"name": "ReserveProductRequest"}], "rpcs": [/* … */]},
 "resources": ["graph://org/inventory-service/commit/abc123/node/handler_reserve"],
 "suggested_next_calls": [{"tool": "explain_symbol", "args": {"symbol": "ReserveProduct"}}]}
```

Once `get_context` identifies the relevant surface, follow `suggested_next_calls` into the
precise tools (`explain_symbol`, `explain_rpc`, `explain_endpoint`, `find_private_library`),
as the next examples show.

---

## 1. "Which handler implements the reserve-product endpoint?"

Find the endpoint, then explain it, then walk into the code.

**Step 1 — find the endpoint** (`find_endpoint`):

```json
// request
{"repo": "org/inventory-service", "query": "reserve product"}
// response (trimmed)
[{"Method": "POST", "Path": "/v1/reservations", "OperationID": "reserveProduct"}]
```

**Step 2 — explain it** (`explain_endpoint`):

```json
// request
{"repo": "org/inventory-service", "method": "POST", "path": "/v1/reservations"}
// response (trimmed)
{"OperationID": "reserveProduct", "RequestSchema": "ReserveProductRequest", "ResponseSchema": "Reservation", "SpecPath": "api/openapi.yaml"}
```

**Step 3 — follow the implementation in code** (`explain_symbol`):

```json
// request
{"repo": "org/inventory-service", "symbol": "ReserveProduct"}
// response (trimmed)
{
  "Node": {"NodeID": "http_reservation_reserveproduct", "SourceFile": "internal/http/reservation_handler.go"},
  "Callees": [{"Source": "http_reservation_reserveproduct", "Target": "reservation_service_reserveproduct", "Relation": "calls"}]
}
```

→ The HTTP handler calls the domain service `ReserveProduct`. Repeat
`explain_symbol` on the callee to walk deeper (service → repository).

---

## 2. "What does the ReserveProduct gRPC RPC look like, and who implements it?"

**Step 1 — find the RPC** (`find_rpc`):

```json
// request
{"repo": "org/inventory-service", "query": "ReserveProduct", "stream_kind": "unary"}
```

**Step 2 — explain it** (`explain_rpc`):

```json
// request
{"repo": "org/inventory-service", "service": "ReservationService", "rpc": "ReserveProduct"}
// response (trimmed)
{
  "rpc": {"Name": "ReserveProduct", "Service": "ReservationService"},
  "request_message_fields": [{"name": "product_id", "type": "string", "number": 1}],
  "implemented_by": [{"NodeID": "reservation_server_reserveproduct", "SourceFile": "internal/grpc/server.go"}],
  "consumed_by": ""
}
```

→ `implemented_by` points at the gRPC server method. `consumed_by` is empty
because cross-repo RPC consumer aggregation is not yet implemented (see
[concepts.md](concepts.md#whats-not-yet-implemented)).

---

## 3. "What breaks if I change ReserveProductRequest?"

This is the flagship cross-cutting question. A thorough answer checks the schema
on **both** contract surfaces and the code that uses them.

```json
// OpenAPI schema usage
find_schema   {"repo": "org/inventory-service", "query": "ReserveProductRequest"}
// proto message (same name, proto side)
explain_rpc   {"repo": "org/inventory-service", "service": "ReservationService", "rpc": "ReserveProduct"}
// code symbols touching it
query_repo    {"repo": "org/inventory-service", "name": "ReserveProductRequest"}
explain_symbol{"repo": "org/inventory-service", "symbol": "ReserveProductRequest"}
```

Combine the results into: **affected contracts** (OpenAPI schema + proto
message), **affected operations/RPCs**, **implementations** (handlers/servers),
and a **risk** read. Cross-repo consumer expansion is the piece not yet implemented.

---

## 4. "Who consumes our auth library, and which symbols?"

**Step 1 — find the library** (`find_private_library`):

```json
// request
{"repo": "org/platform-libs", "query": "auth"}
```

**Step 2 — find consumers + used symbols** (`find_library_consumers`):

```json
// request
{"repo": "org/inventory-service", "module": "github.com/org/platform-libs/auth"}
// response (trimmed)
{
  "module_path": "github.com/org/platform-libs/auth",
  "version": "v1.4.0",
  "used_packages": ["auth", "auth/jwt"],
  "used_symbols": [
    {"PackagePath": "auth", "Symbol": "ValidateToken", "File": "internal/mw/auth.go", "Line": 22}
  ]
}
```

→ You get the pinned version and the exact symbols `org/inventory-service`
imports — enough to assess the blast radius of changing `ValidateToken`.

---

## 5. "Who consumes this library across the org?"

`find_library_consumers` answers for a **single** repo; `explain_private_library`
aggregates **every** indexed repo at once, from both sides.

```json
// request
{"query": "auth"}
```

The response carries `provider` (the defining repo's exports, code-graph linked)
and `consumers` — each consuming repo with its **pinned version** and the
**used symbols**, every usage best-effort linked to the enclosing consumer node.
Because all repos are aggregated in one call, an agent can spot **version skew**
(repos stuck on older versions) and **usage hotspots** (the most-imported symbols)
immediately, then follow `explain_symbol` on a linked consumer node to walk into
the calling code.

---

## 6. "Orient me in an unknown index"

```json
list_repos    {}                                   // what's indexed?
list_apis     {"repo": "org/inventory-service"}    // what contracts does it have?
resolve_repo  {"query": "inv"}                      // fuzzy → candidates
```

`list_apis` returns kind-discriminated entries (`openapi` and `proto`) so one
call surveys both contract surfaces.

---

## Tips for agents

- **Search, then explain, then walk.** `find_*` narrows, `explain_*` enriches,
  `explain_symbol` traverses the code graph one hop at a time.
- **Resolve fuzzy repos first** with `resolve_repo` when the user's phrasing
  doesn't match `org/name` exactly.
- **Cite commit-pinned resources** ([resources.md](resources.md)) for anything
  you found, so the reference stays reproducible.
- **Expect graceful not-found** — probing a missing endpoint returns an error
  result, not a crash.
