# MCP resources reference

Alongside tools, the server exposes **5 resource templates**. Resources are for
**addressable, commit-pinned lookups** of a specific artifact by URI — useful
when an agent already knows the exact identity it wants and prefers a stable
reference over a search. Every resource returns `application/json`.

| Name | URI template |
|------|--------------|
| `repo` | `repo://{org}/{name}` |
| `graph-node` | `graph://{org}/{name}/commit/{sha}/node/{nodeID}` |
| `openapi` | `openapi://{org}/{name}/commit/{sha}/{kind}/{ref}` |
| `proto` | `proto://{org}/{name}/commit/{sha}/{kind}/{ref}` |
| `lib` | `lib://{module}[/version/{v}][/package/{p}][/symbol/{s}]` |

A missing artifact returns a resource-not-found error.

---

## `repo://{org}/{name}`

Repo snapshot summary as JSON.

```
repo://org/inventory-service
```

---

## `graph://{org}/{name}/commit/{sha}/node/{nodeID}`

A single graph node, commit-pinned. The `commit` segment is part of the address
(reproducibility); resolution is by snapshot.

```
graph://org/inventory-service/commit/abc123/node/reservation_service_reserveproduct
```

---

## `openapi://{org}/{name}/commit/{sha}/{kind}/{ref}`

An OpenAPI artifact. `kind` is one of `operation`, `schema`, `spec`. The `ref`
may contain slashes (e.g. a spec path); everything after `kind` is the ref.

```
openapi://org/inventory-service/commit/abc123/operation/reserveProduct
openapi://org/inventory-service/commit/abc123/schema/ReserveProductRequest
openapi://org/inventory-service/commit/abc123/spec/api/openapi.yaml
```

| kind | ref is | backed by |
|------|--------|-----------|
| `operation` | operationId | `OperationResource` |
| `schema` | schema name | `SchemaResource` |
| `spec` | spec path | `SpecResource` |

---

## `proto://{org}/{name}/commit/{sha}/{kind}/{ref}`

A protobuf artifact. `kind` is one of `file`, `service`, `rpc`, `message`. Note
the ref segment counts for the compound kinds:

```
proto://org/reservation-service/commit/def456/file/proto/reservation/v1/reservation.proto
proto://org/reservation-service/commit/def456/service/reservation.v1/ReservationService
proto://org/reservation-service/commit/def456/rpc/reservation.v1/ReservationService/ReserveProduct
proto://org/reservation-service/commit/def456/message/reservation.v1/ReserveProductRequest
```

| kind | ref shape | resolves to |
|------|-----------|-------------|
| `file` | `<proto path>` | `ProtoFileResource` |
| `service` | `<package>/<service>` | `ProtoServiceResource` |
| `rpc` | `<package>/<service>/<rpc>` | `ProtoRPCResource` |
| `message` | `<package>/<message>` | `ProtoMessageResource` |

---

## `lib://{module}[/version/{v}][/package/{p}][/symbol/{s}]`

A private Go library, drillable from module → version → package → symbol. Unlike
the others, `lib://` is **not** commit-pinned — it addresses a module path.

```
lib://github.com/org/platform-libs/auth
lib://github.com/org/platform-libs/auth/version/v1.4.0
lib://github.com/org/platform-libs/auth/package/auth/jwt
lib://github.com/org/platform-libs/auth/symbol/ValidateToken
```

| segment present | resolves to |
|-----------------|-------------|
| (module only) or `/version/…` | `LibraryResource` |
| `/package/<p>` | `LibraryPackageResource` |
| `/symbol/<s>` | `LibrarySymbolResource` |

---

## Tools vs. resources — which to use

| You want to… | Use |
|--------------|-----|
| Search by NL / substring / partial name | a **tool** (`find_*`) |
| Explain something and follow its links | a **tool** (`explain_*`) |
| Fetch a known artifact by exact identity, reproducibly | a **resource** URI |

In practice an agent searches with tools, then can cite a commit-pinned
resource URI for anything it found.
