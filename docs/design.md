# API and Private Library Intelligence Layer

The MCP server should also understand API contracts and private libraries.

This is important because many service relationships are not visible only from source-code symbols. They are often defined through:

* OpenAPI specs
* protobuf files
* generated clients
* internal Go modules
* private shared libraries
* SDKs
* versioned contracts
* dependency files

This layer should answer questions like:

```text
Which service exposes this endpoint?
Which handler implements this OpenAPI operation?
Which proto service owns this RPC?
Which services consume this private library?
What will break if I change this API schema?
Where is this protobuf message used?
Which version of the internal SDK does this repo use?
```

---

## API Sources to Index

The indexer should scan repositories for API-related files.

Examples:

```text
OpenAPI:
  - openapi.yaml
  - openapi.yml
  - openapi.json
  - swagger.yaml
  - swagger.json
  - api/**/*.yaml
  - api/**/*.json

Protocol Buffers:
  - proto/**/*.proto
  - api/**/*.proto
  - internal/**/*.proto

Go dependencies:
  - go.mod
  - go.sum
  - go.work

Other dependency files, later:
  - package.json
  - pnpm-lock.yaml
  - yarn.lock
  - pyproject.toml
  - poetry.lock
  - requirements.txt
  - pom.xml
  - build.gradle
  - Cargo.toml
```

For MVP, focus on:

```text
1. OpenAPI
2. protobuf
3. Go private modules
```

---

## OpenAPI Support

The indexer should parse OpenAPI specs and extract:

```text
- API title
- API version
- servers
- paths
- HTTP methods
- operationId
- tags
- summary
- description
- parameters
- request body schema
- response schemas
- error responses
- auth/security requirements
- referenced components
```

Example normalized operation:

```json
{
  "repo": "org/inventory-service",
  "spec_path": "api/openapi.yaml",
  "api_name": "Inventory API",
  "api_version": "1.4.0",
  "method": "POST",
  "path": "/products/{productId}/reservations",
  "operation_id": "reserveProduct",
  "tags": ["reservations"],
  "summary": "Reserve product stock",
  "request_schema": "ReserveProductRequest",
  "response_schema": "ReservationResponse",
  "security": ["bearerAuth"],
  "implemented_by": [
    "internal/http/reservation_handler.go:ReserveProduct"
  ]
}
```

The useful part is not only parsing the spec.

The useful part is linking the API contract back to the code graph.

Example edges:

```text
OpenAPI operation -> implemented by HTTP handler
OpenAPI operation -> uses request schema
OpenAPI operation -> returns response schema
OpenAPI schema -> maps to Go struct
OpenAPI operation -> calls service method
OpenAPI operation -> depends on private library
```

---

## Protobuf Support

The indexer should parse `.proto` files and extract:

```text
- package name
- imports
- services
- RPC methods
- request messages
- response messages
- message fields
- enum definitions
- options
- generated Go package
```

Example normalized RPC:

```json
{
  "repo": "org/inventory-service",
  "proto_path": "proto/inventory/v1/inventory.proto",
  "package": "inventory.v1",
  "service": "InventoryService",
  "rpc": "ReserveProduct",
  "request_message": "ReserveProductRequest",
  "response_message": "ReserveProductResponse",
  "implemented_by": [
    "internal/grpc/inventory_server.go:ReserveProduct"
  ],
  "consumed_by": [
    "org/warehouse-service",
    "org/order-service"
  ]
}
```

Example graph edges:

```text
Proto service -> has RPC
RPC -> uses request message
RPC -> uses response message
Message -> has fields
Proto file -> imports another proto file
Generated client -> consumed by service
RPC implementation -> calls domain service
```

This makes the MCP server useful for gRPC-heavy systems.

---

## Private Library Support

Private libraries should be first-class indexed objects.

The MCP server should understand both sides:

```text
1. Provider side:
   The repository that defines the private library.

2. Consumer side:
   The repositories that import and use the private library.
```

For Go, the indexer should parse:

```text
- go.mod
- go.sum
- go.work
- import statements
- exported packages
- exported functions
- exported types
- interfaces
- public constructors
```

Example private library record:

```json
{
  "library": "git.company.local/platform/auth",
  "repo": "platform/auth",
  "version": "v1.8.2",
  "packages": [
    "git.company.local/platform/auth/token",
    "git.company.local/platform/auth/middleware"
  ],
  "exports": [
    "token.Validator",
    "token.Parse",
    "middleware.RequireAuth"
  ]
}
```

Example consumer record:

```json
{
  "repo": "org/warehouse-service",
  "dependency": "git.company.local/platform/auth",
  "version": "v1.8.2",
  "used_packages": [
    "git.company.local/platform/auth/token",
    "git.company.local/platform/auth/middleware"
  ],
  "used_symbols": [
    "token.Validator",
    "middleware.RequireAuth"
  ]
}
```

---

## Private Library Questions

The MCP server should answer:

```text
Which repos use this library?
Which version does each repo use?
Which symbols from this library are actually used?
Which exported functions are unused?
What changed between library versions?
Which consumers may break if I change this interface?
Where is this middleware used?
Which service still uses an old version?
```

Example tool:

```text
find_library_consumers
```

Input:

```json
{
  "library": "git.company.local/platform/auth"
}
```

Output:

```json
{
  "library": "git.company.local/platform/auth",
  "consumers": [
    {
      "repo": "org/warehouse-service",
      "version": "v1.8.2",
      "used_symbols": [
        "middleware.RequireAuth",
        "token.Validator"
      ]
    },
    {
      "repo": "org/order-service",
      "version": "v1.7.0",
      "used_symbols": [
        "token.Parse"
      ]
    }
  ]
}
```

---

## Additional MCP Resources

Add API-specific resources:

```text
openapi://org/repo/commit/<sha>/spec/<path>
openapi://org/repo/commit/<sha>/operation/<operationId>
openapi://org/repo/commit/<sha>/schema/<schemaName>

proto://org/repo/commit/<sha>/file/<path>
proto://org/repo/commit/<sha>/service/<package>/<service>
proto://org/repo/commit/<sha>/rpc/<package>/<service>/<rpc>
proto://org/repo/commit/<sha>/message/<package>/<message>

lib://<module-path>
lib://<module-path>/version/<version>
lib://<module-path>/package/<package-path>
lib://<module-path>/symbol/<symbol>
```

Examples:

```text
openapi://org/inventory-service/commit/abc123/operation/reserveProduct
proto://org/inventory-service/commit/abc123/rpc/inventory.v1/InventoryService/ReserveProduct
lib://git.company.local/platform/auth/version/v1.8.2
```

---

## Additional MCP Tools

### `list_apis`

Lists API contracts found in a repository.

Input:

```json
{
  "repo": "org/inventory-service",
  "branch": "main"
}
```

Output:

```json
{
  "apis": [
    {
      "kind": "openapi",
      "name": "Inventory API",
      "version": "1.4.0",
      "path": "api/openapi.yaml"
    },
    {
      "kind": "protobuf",
      "package": "inventory.v1",
      "path": "proto/inventory/v1/inventory.proto"
    }
  ]
}
```

---

### `find_endpoint`

Finds an HTTP endpoint by natural language, path, method or operation ID.

Input:

```json
{
  "repo": "org/inventory-service",
  "query": "reserve product stock"
}
```

Output:

```json
{
  "matches": [
    {
      "method": "POST",
      "path": "/products/{productId}/reservations",
      "operation_id": "reserveProduct",
      "spec_path": "api/openapi.yaml",
      "implemented_by": [
        "internal/http/reservation_handler.go:ReserveProduct"
      ]
    }
  ]
}
```

---

### `explain_endpoint`

Explains an HTTP endpoint from OpenAPI and source code.

Input:

```json
{
  "repo": "org/inventory-service",
  "method": "POST",
  "path": "/products/{productId}/reservations"
}
```

Output:

```json
{
  "summary": "This endpoint reserves stock for a product.",
  "operation_id": "reserveProduct",
  "request_schema": "ReserveProductRequest",
  "response_schema": "ReservationResponse",
  "handler": "ReservationHandler.ReserveProduct",
  "service_flow": [
    "ReservationHandler.ReserveProduct",
    "ReservationService.ReserveProduct",
    "ReservationRepository.Create"
  ],
  "related_files": [
    "api/openapi.yaml",
    "internal/http/reservation_handler.go",
    "internal/reservation/service.go",
    "internal/reservation/repository.go"
  ]
}
```

---

### `find_rpc`

Finds a protobuf RPC by natural language, service name or method name.

Input:

```json
{
  "repo": "org/inventory-service",
  "query": "reserve product"
}
```

Output:

```json
{
  "matches": [
    {
      "package": "inventory.v1",
      "service": "InventoryService",
      "rpc": "ReserveProduct",
      "request": "ReserveProductRequest",
      "response": "ReserveProductResponse",
      "proto_path": "proto/inventory/v1/inventory.proto"
    }
  ]
}
```

---

### `explain_rpc`

Explains a protobuf RPC and links it to implementation code.

Input:

```json
{
  "repo": "org/inventory-service",
  "package": "inventory.v1",
  "service": "InventoryService",
  "rpc": "ReserveProduct"
}
```

Output:

```json
{
  "summary": "ReserveProduct reserves stock for a product through the inventory gRPC API.",
  "request_message": "ReserveProductRequest",
  "response_message": "ReserveProductResponse",
  "implemented_by": [
    "internal/grpc/inventory_server.go:ReserveProduct"
  ],
  "calls": [
    "ReservationService.ReserveProduct"
  ],
  "consumed_by": [
    "org/warehouse-service",
    "org/order-service"
  ]
}
```

---

### `find_schema`

Finds an OpenAPI schema or protobuf message.

Input:

```json
{
  "repo": "org/inventory-service",
  "query": "reservation response"
}
```

Output:

```json
{
  "matches": [
    {
      "kind": "openapi_schema",
      "name": "ReservationResponse",
      "path": "api/openapi.yaml"
    },
    {
      "kind": "proto_message",
      "name": "ReserveProductResponse",
      "path": "proto/inventory/v1/inventory.proto"
    }
  ]
}
```

---

### `compare_api_versions`

Compares two versions of an API contract.

Input:

```json
{
  "repo": "org/inventory-service",
  "from": "v1.4.0",
  "to": "v1.5.0",
  "api": "Inventory API"
}
```

Output:

```json
{
  "summary": "The new version adds one endpoint and changes the ReservationResponse schema.",
  "added_endpoints": [
    "GET /reservations/{reservationId}"
  ],
  "changed_schemas": [
    {
      "schema": "ReservationResponse",
      "changes": [
        "added field expiresAt",
        "changed field status enum values"
      ]
    }
  ],
  "possible_breaking_changes": [
    "ReservationStatus enum changed"
  ]
}
```

---

### `find_private_library`

Finds an internal library by name, module path or purpose.

Input:

```json
{
  "query": "auth middleware jwt validation"
}
```

Output:

```json
{
  "matches": [
    {
      "library": "git.company.local/platform/auth",
      "repo": "platform/auth",
      "reason": "Provides JWT validation and HTTP middleware."
    }
  ]
}
```

---

### `explain_private_library`

Explains what a private library provides.

Input:

```json
{
  "library": "git.company.local/platform/auth"
}
```

Output:

```json
{
  "summary": "Internal authentication library for token validation and HTTP middleware.",
  "packages": [
    {
      "path": "git.company.local/platform/auth/token",
      "purpose": "JWT parsing and validation"
    },
    {
      "path": "git.company.local/platform/auth/middleware",
      "purpose": "HTTP middleware for auth enforcement"
    }
  ],
  "main_exports": [
    "token.Validator",
    "token.Parse",
    "middleware.RequireAuth"
  ],
  "consumers": [
    "org/warehouse-service",
    "org/order-service"
  ]
}
```

---

### `impact_api_change`

Estimates impact of an API or protobuf change.

Input:

```json
{
  "repo": "org/inventory-service",
  "change": "remove field productId from ReserveProductRequest"
}
```

Output:

```json
{
  "summary": "Removing productId is likely breaking because it is required by OpenAPI and used by two consumers.",
  "affected_contracts": [
    "api/openapi.yaml",
    "proto/inventory/v1/inventory.proto"
  ],
  "affected_implementations": [
    "internal/http/reservation_handler.go",
    "internal/grpc/inventory_server.go",
    "internal/reservation/service.go"
  ],
  "affected_consumers": [
    "org/warehouse-service",
    "org/order-service"
  ],
  "risk": "high"
}
```

---

## API Graph Model

Extend the existing graph model with API-specific nodes.

New node types:

```text
api_spec
http_operation
http_schema
proto_file
proto_package
proto_service
proto_rpc
proto_message
proto_enum
private_library
private_library_version
private_package
private_symbol
dependency_usage
```

New edge types:

```text
defines
imports
uses_schema
uses_message
implemented_by
calls
consumes
depends_on
exports
uses_symbol
generated_from
version_of
```

Example graph:

```text
OpenAPI operation: POST /products/{productId}/reservations
   -> implemented_by ReservationHandler.ReserveProduct
   -> uses_schema ReserveProductRequest
   -> uses_schema ReservationResponse
   -> calls ReservationService.ReserveProduct

Proto RPC: InventoryService.ReserveProduct
   -> uses_message ReserveProductRequest
   -> uses_message ReserveProductResponse
   -> implemented_by InventoryServer.ReserveProduct
   -> calls ReservationService.ReserveProduct

Warehouse service
   -> consumes InventoryService.ReserveProduct
   -> depends_on git.company.local/platform/auth@v1.8.2
   -> uses_symbol middleware.RequireAuth
```

This gives the AI agent a better understanding of service boundaries.

---

## API-Aware Retrieval Strategy

For API-related questions, retrieval should use contract files first.

Flow:

```text
1. Detect whether the question is about HTTP API, gRPC, schema, SDK or library usage.
2. Search OpenAPI specs, proto files and dependency files.
3. Match operationId, path, RPC name, message name or library module path.
4. Link the contract to implementation through Graphify nodes.
5. Expand to handler, service, repository and client code.
6. Search consumer repositories when needed.
7. Return a compact answer with contracts, implementation and consumers.
```

Example:

```text
User asks:
"How does warehouse reserve inventory?"

MCP should return:
1. warehouse client code
2. consumed proto RPC or OpenAPI endpoint
3. inventory service handler/server implementation
4. inventory domain service
5. relevant request/response schemas
6. private libraries involved in auth/client generation
```

---

## Storage Extension

Add tables for API contracts and private libraries.

```sql
CREATE TABLE api_specs (
    id INTEGER PRIMARY KEY,
    index_id INTEGER NOT NULL,
    kind TEXT NOT NULL,
    name TEXT,
    version TEXT,
    path TEXT NOT NULL
);

CREATE TABLE http_operations (
    id INTEGER PRIMARY KEY,
    api_spec_id INTEGER NOT NULL,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    operation_id TEXT,
    summary TEXT,
    request_schema TEXT,
    response_schema TEXT,
    security TEXT
);

CREATE TABLE api_schemas (
    id INTEGER PRIMARY KEY,
    api_spec_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    raw_ref TEXT
);

CREATE TABLE proto_files (
    id INTEGER PRIMARY KEY,
    index_id INTEGER NOT NULL,
    path TEXT NOT NULL,
    package_name TEXT
);

CREATE TABLE proto_services (
    id INTEGER PRIMARY KEY,
    proto_file_id INTEGER NOT NULL,
    name TEXT NOT NULL
);

CREATE TABLE proto_rpcs (
    id INTEGER PRIMARY KEY,
    proto_service_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    request_message TEXT NOT NULL,
    response_message TEXT NOT NULL
);

CREATE TABLE proto_messages (
    id INTEGER PRIMARY KEY,
    proto_file_id INTEGER NOT NULL,
    name TEXT NOT NULL
);

CREATE TABLE dependencies (
    id INTEGER PRIMARY KEY,
    index_id INTEGER NOT NULL,
    ecosystem TEXT NOT NULL,
    name TEXT NOT NULL,
    version TEXT,
    is_private BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE private_library_exports (
    id INTEGER PRIMARY KEY,
    index_id INTEGER NOT NULL,
    module_path TEXT NOT NULL,
    package_path TEXT NOT NULL,
    symbol_name TEXT NOT NULL,
    symbol_kind TEXT
);

CREATE TABLE private_library_usages (
    id INTEGER PRIMARY KEY,
    index_id INTEGER NOT NULL,
    module_path TEXT NOT NULL,
    package_path TEXT,
    symbol_name TEXT,
    file_path TEXT,
    line_number INTEGER
);
```

---

## Example End-to-End Query

User asks:

```text
What happens if I change ReserveProductRequest?
```

The MCP server should check:

```text
1. OpenAPI schemas named ReserveProductRequest
2. Proto messages named ReserveProductRequest
3. HTTP operations using the schema
4. RPCs using the message
5. Go structs generated from the schema/message
6. Handler/server implementations
7. Client code in consumer services
8. Private libraries or SDKs generated from the contract
9. Tests touching this request
```

Output should look like:

```json
{
  "summary": "ReserveProductRequest is used by both HTTP and gRPC reservation APIs. Changing it may affect warehouse-service and order-service.",
  "contracts": [
    "api/openapi.yaml#/components/schemas/ReserveProductRequest",
    "proto/inventory/v1/inventory.proto:ReserveProductRequest"
  ],
  "operations": [
    "POST /products/{productId}/reservations",
    "InventoryService.ReserveProduct"
  ],
  "implementations": [
    "internal/http/reservation_handler.go",
    "internal/grpc/inventory_server.go",
    "internal/reservation/service.go"
  ],
  "consumers": [
    "org/warehouse-service",
    "org/order-service"
  ],
  "risk": "high"
}
```

---

## Updated MVP Scope

Extended MVP:

```text
1. Existing Graphify loader
2. SQLite storage
3. MCP stdio server
4. OpenAPI parser
5. protobuf parser
6. Go module parser
7. Tools:
   - list_repos
   - resolve_repo
   - query_repo
   - explain_symbol
   - get_file_context
   - list_apis
   - find_endpoint
   - explain_endpoint
   - find_rpc
   - explain_rpc
   - find_private_library
   - find_library_consumers
8. Resources:
   - repo://...
   - graph://...
   - openapi://...
   - proto://...
   - lib://...
```

Keep these for later:

```text
- automatic breaking-change detection
- generated client analysis
- multi-language dependency support
- API diff visualizations
- PR review automation
- automatic SDK generation
```

---

## Final Direction

The MCP server should become a private engineering knowledge layer.

Graphify gives code structure.

The API layer gives service contracts.

The private library layer gives internal dependency knowledge.

Together, the agent can answer:

```text
What does this service expose?
What does this service consume?
Which private libraries does it depend on?
Which repos use this API?
Which repos use this library?
Where is this contract implemented?
What breaks if I change this request, response, RPC or exported type?
```

That is the useful shape:

```text
Code graph
+ API contracts
+ protobuf contracts
+ private library usage
+ dependency versions
= private repo context that is actually useful for AI coding agents
```
