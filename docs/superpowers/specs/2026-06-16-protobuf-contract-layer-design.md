---
comet_change: protobuf-contract-layer
role: technical-design
canonical_spec: openspec
---

# Protobuf Contract Layer — Technical Design

## Context

This is split change 3 of 4 of the MVP. It adds a protobuf contract layer to the
intel MCP server: parse `.proto` contracts into storage, link each RPC to its
gRPC server implementation **within the same repo**, and serve the result through
MCP tools and resources.

The design is grounded in the shipped `openapi-contract-layer` (now archived). Two
facts from that layer shape this one:

- **Discovery is manifest-declared, never globbed** (`openapi-index` spec: "SHALL
  NOT auto-discover specs by globbing the filesystem"). Protobuf follows the same
  discipline.
- **The OpenAPI layer deferred all contract→code linking.** No linker, no
  `implemented_by`/`calls`/`consumed_by` edges, and no merged multi-repo graph
  exist yet. Contracts live in dedicated `index_id`-scoped tables, separate from
  the Graphify `nodes`/`edges` tables.

Therefore this change is the first to build a linker. To keep risk bounded, it
builds **same-repo linking only** and **defers cross-repo `consumed_by`** to a
dedicated future change.

## Scope

In scope:

- Parse `.proto` files → `proto_files`, `proto_services`, `proto_rpcs`,
  `proto_messages`, `proto_enums`.
- Same-repo RPC→gRPC-server-impl linking (`proto_rpc_impls` link table).
- `uses_message` as the RPC's resolvable request/response message reference.
- Tools `find_rpc`, `explain_rpc`; additive extensions to `list_apis`, `find_schema`.
- `proto://` resources.

Out of scope (deferred):

- **Cross-repo `consumed_by`** — needs a merged multi-repo graph and
  generated-stub call-site matching that does not exist yet. `explain_rpc` returns
  an explicit "deferred / not available in this change" marker for this field, not
  an error.
- Breaking-change detection, API diffing, generated-client analysis, SDK
  generation (MVP-wide deferrals).

## Architecture

```
manifest proto:{roots,files}      protobuf-contract-layer
 ┌──────────────────────┐  parse  ┌────────────────────────────────────────────┐
 │ entry .proto files    │ ──────▶ │ internal/proto (protocompile)               │
 │ + import roots        │         │   → proto_files / proto_services /          │
 └──────────────────────┘         │     proto_rpcs / proto_messages / proto_enums│
                                   │            │                                 │
 graph.json (Graphify) ──load────▶ │  nodes/edges (existing)                     │
                                   │            │ link (after graph.Load)          │
                                   │            ▼                                  │
                                   │  internal/link → proto_rpc_impls             │
                                   │  (RPC → gRPC server method node, confidence) │
                                   │  tools: find_rpc, explain_rpc;               │
                                   │  extends list_apis, find_schema              │
                                   │  resources: proto://…                        │
                                   └────────────────────────────────────────────┘
```

Ingestion order in `ingest.Run` (per repo): open + parse graph.json →
`graph.Load` (populates nodes/edges) → parse protos → run linker (needs nodes) →
persist proto bundles + impl links. This mirrors the existing OpenAPI pass, which
already runs after `graph.Load`.

## Components

### internal/proto (parser)

New package mirroring `internal/openapi`. Responsibilities:

- Build a `bufbuild/protocompile` compiler with a `SourceResolver` rooted at the
  manifest `roots`, plus the bundled well-known types (`google/protobuf/*`).
- Compile the manifest `files` (expanding directory entries to the `.proto` files
  beneath them) to resolved `protoreflect.FileDescriptor`s.
- Normalize each file descriptor into plain structs:
  - file: repo-relative path, proto `package`, `go_package` option, imports list.
  - service: name, full name (`package.Service`).
  - rpc: name, full name (`package.Service.Rpc`), fully-qualified request and
    response message names, `stream_kind`.
  - message: short name, full name (nested via dotted name), fields
    `[{name, type, number, label}]`.
  - enum: short name, full name, values `[{name, number}]`.

`stream_kind` is derived from the method descriptor's
`IsStreamingClient`/`IsStreamingServer`:

| client stream | server stream | stream_kind     |
|---------------|---------------|-----------------|
| false         | false         | `unary`         |
| false         | true          | `server_stream` |
| true          | false         | `client_stream` |
| true          | true          | `bidi`          |

Error tolerance matches OpenAPI: a missing or malformed proto file (or unresolved
import) is skipped with a warning recorded on the ingest `Report`; the run
continues. Compilation is best-effort per file set.

### internal/store (storage)

New `internal/store/proto.go` plus tables added to `schema.go`. All tables are
`index_id`-scoped and follow the `api_specs` family conventions.

```sql
CREATE TABLE proto_files (
  id INTEGER PRIMARY KEY,
  index_id INTEGER NOT NULL REFERENCES indexes(id),
  path TEXT NOT NULL,
  package TEXT,
  go_package TEXT,
  imports TEXT          -- JSON array of imported paths
);
CREATE TABLE proto_services (
  id INTEGER PRIMARY KEY,
  proto_file_id INTEGER NOT NULL REFERENCES proto_files(id),
  name TEXT NOT NULL,
  full_name TEXT NOT NULL
);
CREATE TABLE proto_rpcs (
  id INTEGER PRIMARY KEY,
  proto_service_id INTEGER NOT NULL REFERENCES proto_services(id),
  name TEXT NOT NULL,
  full_name TEXT NOT NULL,
  request_message TEXT,   -- fully-qualified message name
  response_message TEXT,  -- fully-qualified message name
  stream_kind TEXT NOT NULL  -- unary|server_stream|client_stream|bidi
);
CREATE TABLE proto_messages (
  id INTEGER PRIMARY KEY,
  proto_file_id INTEGER NOT NULL REFERENCES proto_files(id),
  name TEXT NOT NULL,
  full_name TEXT NOT NULL,
  fields TEXT             -- JSON array [{name,type,number,label}]
);
CREATE TABLE proto_enums (
  id INTEGER PRIMARY KEY,
  proto_file_id INTEGER NOT NULL REFERENCES proto_files(id),
  name TEXT NOT NULL,
  full_name TEXT NOT NULL,
  values TEXT            -- JSON array [{name,number}]
);
CREATE TABLE proto_rpc_impls (
  id INTEGER PRIMARY KEY,
  proto_rpc_id INTEGER NOT NULL REFERENCES proto_rpcs(id),
  node_id TEXT NOT NULL,        -- Graphify code node in the SAME index
  confidence REAL NOT NULL,     -- 0.0–1.0
  match_reason TEXT             -- which signals fired
);
```

Indexing is idempotent per `index_id`: a `ReplaceProtoFiles(indexID, bundles)`
transaction deletes this index's proto rows (children first) and re-inserts, with
identical row counts on a second run. `proto_rpc_impls` rows are written by the
linker after parse, scoped to the same index, and cleared on re-index.

`uses_message` is **not** a separate table: an RPC's `request_message` /
`response_message` are fully-qualified names resolvable to a `proto_messages` row
by `full_name` within the index — a 1:1 structural reference. Resolution happens at
query time in `explain_rpc` and the message resource.

### internal/link (linker — new shared package)

The first linker in the codebase, deliberately placed in a shared package so the
OpenAPI handler linker can adopt it later. It maps a proto RPC to 0..n code-method
nodes in the same index, each with a confidence tier and reason.

Inputs: the parsed services/RPCs for a repo and the loaded graph nodes/edges for
the same `index_id`. The Graphify `nodes` row exposes `label`, `file_type`,
`source_file`, `source_location`, `source_url` (see `internal/graph`).

Matching signals:

1. **Name match** — candidate method nodes whose `label` equals the RPC name.
2. **Service association** — presence in the repo of the generated
   `Register<Service>Server` registration symbol and/or the `<Service>Server`
   interface symbol, used to associate a candidate method with its service.
3. **Streaming-aware signature check** — when the candidate's source is readable
   (`source_file` + `source_location`), inspect the method's parameter shape and
   compare against the RPC's `stream_kind`: unary is `(ctx, *Req) (*Resp, error)`;
   streaming variants take a generated stream type (e.g. bidi
   `(<Svc>_<Rpc>Server) error`). A match disambiguates overloaded names and raises
   confidence.

Confidence tiers:

- **HIGH (≈0.9)** — name match + service association + signature match.
- **MEDIUM (≈0.6)** — name match + service present, weaker/heuristic association.
- **LOW (≈0.3)** — name-only match, or multiple colliding candidates.

Ambiguous matches are **recorded at lower confidence, never silently dropped and
never guessed into a single false-positive**. `match_reason` records the signals
that fired (e.g. `name+service+signature`). If signatures are not readable from
the graph, the linker falls back to name+service matching at reduced confidence;
the RPC's `stream_kind` is always recorded and surfaced regardless.

### internal/mcp (tools + resources)

New `internal/mcp/proto_tools.go`; resource handlers added to `resources.go`.

- `find_rpc(repo, query, stream_kind?)` → list of
  `{full_name, service, rpc, request_message, response_message, stream_kind, proto_path}`.
  Lexical (case-insensitive substring) match on rpc/service/full_name/package,
  optionally filtered to a `stream_kind`.
- `explain_rpc(repo, service, rpc)` →
  `{service, rpc, full_name, stream_kind, request_message, response_message,
    request_message_fields, response_message_fields, implemented_by[], calls[],
    consumed_by}` where:
  - `implemented_by[]` = `{node_id, source_file, confidence, match_reason}` from
    `proto_rpc_impls`.
  - `calls[]` = best-effort **one-hop** outgoing graph edges from the best impl
    node (reusing edges Graphify already produced), marked best-effort.
  - `consumed_by` = the explicit string marker
    `"deferred: cross-repo consumed_by not available in this change"`.
  - Unknown service/rpc returns a structured not-found result, not an error/crash.
- `list_apis` gains `{kind:"protobuf", name, version:"", path}` entries alongside
  existing `{kind:"openapi", …}` entries. HTTP output is unchanged.
- `find_schema` gains `{kind:"proto_message", name, spec_path}` entries alongside
  existing `{kind:"openapi_schema", …}` entries.

Resources (commit-pinned from manifest metadata, degrading to branch then latest,
like `openapi://`):

```
proto://org/name/commit/<sha>/file/<path>
proto://org/name/commit/<sha>/service/<package>/<service>
proto://org/name/commit/<sha>/rpc/<package>/<service>/<rpc>
proto://org/name/commit/<sha>/message/<package>/<message>
```

### internal/config (manifest)

`RepoConfig` gains a protobuf block:

```go
Proto struct {
    Roots []string `mapstructure:"roots"` // import-resolution roots
    Files []string `mapstructure:"files"` // entry .proto files or dirs to index
} `mapstructure:"proto"`
```

A repo with no `proto:` block indexes zero proto files and no error, exactly like a
repo with no `openapi:` list.

## Data Flow (example)

`explain_rpc("acme/inventory", "InventoryService", "ReserveProduct")`:

1. Resolve repo → `index_id`.
2. Look up the RPC by service+name → proto facts + `stream_kind`,
   `request_message=acme.inventory.ReserveProductRequest`,
   `response_message=acme.inventory.ReserveProductResponse`.
3. Resolve request/response message names → `proto_messages` rows → fields.
4. Read `proto_rpc_impls` for the RPC → `implemented_by[]` with confidence/reason.
5. From the best impl `node_id`, read one-hop outgoing edges → `calls[]`.
6. `consumed_by` → deferred marker.
7. Return the assembled JSON.

## Error Handling

- Missing/malformed/​unresolvable proto file → skipped with a warning on the ingest
  `Report`; the rest of the run continues.
- Unknown RPC/message in a tool or resource → structured not-found, never a crash.
- Linker finds no impl → `implemented_by` is empty; this is a valid result, not an
  error.
- `find_schema`/`list_apis` always return their existing HTTP entries even if the
  proto path errors, so HTTP behavior never regresses.

## Testing Strategy

- **Parser unit tests** over fixture proto sets: cross-file imports, nested
  messages, enums, options, `go_package`, and all four `stream_kind` values.
- **Storage tests**: round-trip + idempotency (re-index → identical proto row
  counts; old `proto_rpc_impls` cleared).
- **Linker tests** on a fixture repo containing generated gRPC stubs + an impl
  type: assert HIGH-confidence impl links for matching methods, streaming-aware
  signature disambiguation, and **no false-positive** impl on an unrelated
  same-named method; assert ambiguous case is recorded at LOW confidence.
- **Tool/resource tests**: `find_rpc` lexical + `stream_kind` filter; `explain_rpc`
  impl + one-hop calls + deferred `consumed_by` marker; not-found behavior.
- **Regression**: `list_apis`/`find_schema` HTTP output unchanged (golden compare
  against the OpenAPI-only result).

## Decisions and Rationale

- **Same-repo linking, defer cross-repo `consumed_by`.** The cross-repo path is
  research-grade (merged graph + stub recognition + confidence) and has no existing
  infrastructure; shipping same-repo linking first delivers value and builds the
  reusable linker.
- **Manifest discovery, no globbing.** Consistency with `openapi-index` and
  deterministic, reproducible indexing; proto imports also need explicit roots.
- **bufbuild/protocompile.** Actively maintained successor to jhump/protoparse;
  fully-resolved descriptors give message refs, enums, options, and `go_package`
  directly.
- **Streaming as `stream_kind` enum + filter + signature-aware linking.** The
  realistic maximum a static index can model; lifecycle/backpressure has no meaning
  without a runtime.
- **Dedicated tables + a link table, not graph nodes/edges for proto entities.**
  Matches the shipped OpenAPI separation; the link table references code nodes by
  `node_id` without forcing proto entities into the graph tables.
- **`internal/link` as a shared package.** Framed as reusable infra the OpenAPI
  handler linker can adopt later.

## Spec Patches (written back to OpenSpec delta specs)

The change had no delta specs; this design authors them:

- `protobuf-index` (ADDED): manifest discovery, protocompile parse, `stream_kind`,
  same-repo RPC→impl + `uses_message` linking, idempotency, tolerance.
- `protobuf-tools` (ADDED): `find_rpc` (+`stream_kind` filter), `explain_rpc`
  (impl + one-hop calls; `consumed_by` deferred marker), `proto://` resources.
- `openapi-tools` (MODIFIED): `list_apis` and `find_schema` requirements updated to
  include protobuf-kind entries (additive; HTTP output unchanged).
