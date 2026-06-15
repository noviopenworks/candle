---
comet_change: openapi-contract-layer
role: technical-design
canonical_spec: openspec
---

# Technical Design — openapi-contract-layer

Adds an OpenAPI contract layer on top of the archived `mcp-core-foundation`: parse OpenAPI
specs into SQLite (tied to the foundation's `index_id`) and serve them through MCP tools and
`openapi://` resources.

> Canonical capability specs: `openspec/changes/openapi-contract-layer/specs/{openapi-index,openapi-tools}/spec.md`.

## Scope decision (divergence from proposal)

The proposal framed operation→**handler linking** (and `explain_endpoint.service_flow`) as a
core capability. **This change descopes linking** and delivers **pure contract serving**:
parse specs and serve their data. Rationale: chi route path strings are not present in the
Graphify graph, so precise linking requires Go-source AST parsing of chi route registrations
— a meaningfully larger, riskier effort. Linking is **deferred to a dedicated future change**.
`proposal.md` is left as the original intent; the delta specs and this document reflect the
actual (reduced) scope.

## Builds on the foundation

Reuses, unchanged: `registry.Resolve` (repo → `index_id`), the `repos`/`indexes` tables, the
pure-Go SQLite store, the official MCP Go SDK adapter pattern, and the `ErrNotFound`
empty-not-error convention. OpenAPI tools register alongside the five base tools in the same
server.

## Manifest extension

```yaml
repos:
  - repo: org/inventory-service
    graph: /abs/.../graphify-out/graph.json
    commit: abc123
    branch: main
    openapi:                 # NEW — explicit spec paths (no globbing)
      - api/openapi.yaml
```

`RepoConfig` gains `OpenAPI []string` (`mapstructure:"openapi"`). Paths are resolved relative
to the manifest file's directory when not absolute. Repos without `openapi:` simply index no
specs.

## Parser (`internal/openapi`)

- Library: `github.com/getkin/kin-openapi/openapi3`.
- Supports **OpenAPI 3.0 / 3.1**. A document detected as Swagger 2.0 (`swagger: "2.0"`) is
  **skipped with a warning** (deferred).
- Resolve `$ref` and flatten components so schema names are stable identifiers.
- Normalize each spec to:
  - spec meta: `{kind:"openapi", name (info.title), version (info.version), path}`
  - operations: one per `(path, method)` → `{method, path, operation_id, summary, request_schema, response_schema, security, tags}`. `request_schema`/`response_schema` are schema **names/refs** (strings) — not resolved Go structs (that mapping belonged to the deferred linking work).
  - schemas: each `components.schemas` entry → `{name, kind:"openapi_schema", raw_ref}`.

## Storage (new tables)

```sql
CREATE TABLE api_specs (
  id        INTEGER PRIMARY KEY,
  index_id  INTEGER NOT NULL REFERENCES indexes(id),
  kind      TEXT NOT NULL,            -- "openapi"
  name      TEXT,
  version   TEXT,
  path      TEXT NOT NULL
);
CREATE TABLE http_operations (
  id              INTEGER PRIMARY KEY,
  api_spec_id     INTEGER NOT NULL REFERENCES api_specs(id),
  method          TEXT NOT NULL,
  path            TEXT NOT NULL,
  operation_id    TEXT,
  summary         TEXT,
  request_schema  TEXT,
  response_schema TEXT,
  security        TEXT,               -- JSON-encoded list
  tags            TEXT                 -- JSON-encoded list
);
CREATE TABLE api_schemas (
  id          INTEGER PRIMARY KEY,
  api_spec_id INTEGER NOT NULL REFERENCES api_specs(id),
  name        TEXT NOT NULL,
  kind        TEXT NOT NULL,          -- "openapi_schema"
  raw_ref     TEXT
);
CREATE INDEX idx_http_ops_spec    ON http_operations(api_spec_id);
CREATE INDEX idx_http_ops_opid    ON http_operations(operation_id);
CREATE INDEX idx_api_schemas_spec ON api_schemas(api_spec_id);
CREATE INDEX idx_api_specs_index  ON api_specs(index_id);
```

Nullable TEXT columns are read with `COALESCE(col,'')` (the NULL-scan lesson from the
foundation). These tables are added to the foundation's migration `schemaSQL`.

## Indexing

Extend the `index` flow: for each manifest repo, after graph ingest, resolve its `index_id`,
then for each `openapi:` path parse + normalize + persist under that `index_id`. Idempotent:
within a transaction, delete `api_schemas`/`http_operations`/`api_specs` for the repo's
`index_id` (cascade by `api_spec_id`), then insert. Degradation: missing file → warn+skip;
Swagger 2.0 → warn+skip; malformed spec → warn+skip; none abort the run.

## Tools (pure functions over the store)

| Tool | Input | Output |
|------|-------|--------|
| `list_apis` | `{repo}` | `[{kind:"openapi", name, version, path}]` (kind discriminator → protobuf extends later) |
| `find_endpoint` | `{repo, query}` | operations matched lexically on NL / path / method / operationId, ranked |
| `explain_endpoint` | `{repo, method, path}` | `{summary, operation_id, request_schema, response_schema, security, tags, spec_path}` — contract only |
| `find_schema` | `{repo, query}` | matching `{kind:"openapi_schema", name, spec_path}` |

No `implemented_by`, no `service_flow` (deferred). Unknown repo/endpoint/schema → empty result
or `ErrNotFound`, never a crash. `list_apis` output keeps a `kind` field so the protobuf layer
adds entries without breaking shape.

## Resources

- `openapi://org/name/commit/<sha>/spec/<path>` → spec meta + its operations.
- `openapi://org/name/commit/<sha>/operation/<operationId>` → one operation.
- `openapi://org/name/commit/<sha>/schema/<schemaName>` → one schema.

Commit pinned from manifest; degrade to branch/`latest` (same rule as foundation resources).

## Testing strategy

- **Unit**: parser (fixture `openapi.yaml` → normalized operations/schemas; `$ref` resolution), store CRUD for `api_*`, idempotent re-index.
- **Golden**: each tool's JSON output against a committed fixture spec.
- **Degradation**: missing spec file, Swagger 2.0 (skip+warn), malformed spec (skip+warn), unknown repo/endpoint/schema (empty/ErrNotFound).
- **E2E**: index a fixture repo with a spec, serve over stdio, call `list_apis` + `explain_endpoint`, assert contract data.

## Out of scope (deferred)

Operation→handler linking, `service_flow`, schema→Go-struct mapping, Swagger 2.0, request/response
example generation, multi-spec dedup across repos.
