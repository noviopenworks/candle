# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Status: greenfield

This repository is **empty** as of writing. The architecture below is the design
target, not existing code. There are no build/lint/test commands yet — establish
them when the first code lands and update this file. Do not assume any file or
command described here exists until you've verified it.

The complete design spec (all tool I/O examples, full DDL, graph edge catalog,
end-to-end query walkthrough) lives in **`docs/design.md`** — treat it as the
source of truth; this file is the condensed orientation.

## What this is

A **private engineering knowledge layer** delivered as an **MCP server (stdio)**.
It combines three layers so an AI coding agent can reason about service boundaries
across many repos:

1. **Code graph** — produced by an existing **Graphify** loader (code structure: symbols, calls, files).
2. **API contract layer** — OpenAPI specs and protobuf, parsed and linked back to code.
3. **Private library layer** — internal Go modules, both provider side (defines the lib) and consumer side (imports it).

The core value is **not parsing** the contracts — it's **linking contracts back to
the code graph** so questions like "which handler implements this OpenAPI operation?"
or "what breaks if I change this proto message?" resolve across repos.

## MVP scope (build in this order)

Contract parsers cover three ecosystems for MVP; everything else is deferred.

1. Graphify loader integration (consumes existing code-graph output)
2. SQLite storage
3. MCP stdio server
4. OpenAPI parser
5. protobuf parser
6. Go module parser (`go.mod` / `go.sum` / `go.work` + import/export analysis)

**Deferred (do not build for MVP):** automatic breaking-change detection, generated
client analysis, multi-language dependency support, API diff visualizations, PR
review automation, SDK generation. Other dependency ecosystems (npm, pyproject,
Maven/Gradle, Cargo) come *after* the Go path works.

## Architecture: the three sources → one graph

The indexer scans repos for contract files and normalizes them into graph nodes
that connect to Graphify's code symbols.

**API file sources to scan:**
- OpenAPI: `openapi.{yaml,yml,json}`, `swagger.{yaml,json}`, `api/**/*.{yaml,json}`
- Protobuf: `proto/**/*.proto`, `api/**/*.proto`, `internal/**/*.proto`
- Go deps: `go.mod`, `go.sum`, `go.work`

**Graph model** — extend Graphify's existing nodes/edges with API nodes:
- Nodes: `api_spec`, `http_operation`, `http_schema`, `proto_file`, `proto_package`,
  `proto_service`, `proto_rpc`, `proto_message`, `proto_enum`, `private_library`,
  `private_library_version`, `private_package`, `private_symbol`, `dependency_usage`
- Edges: `defines`, `imports`, `uses_schema`, `uses_message`, `implemented_by`,
  `calls`, `consumes`, `depends_on`, `exports`, `uses_symbol`, `generated_from`,
  `version_of`

The linking edges (`implemented_by`, `calls`, `consumes`, `uses_symbol`) are what
make this useful — they bridge a contract node to a Graphify code symbol or to
another repo. An OpenAPI operation links to its handler symbol, which `calls` a
domain service symbol; a proto RPC links to its gRPC server impl and to the
consumer repos that `consume` it.

### Key normalization shapes

- **OpenAPI operation** → `{repo, spec_path, api_name, api_version, method, path, operation_id, tags, request_schema, response_schema, security, implemented_by[]}`
- **Proto RPC** → `{repo, proto_path, package, service, rpc, request_message, response_message, implemented_by[], consumed_by[]}`
- **Private library (provider)** → `{library, repo, version, packages[], exports[]}`
- **Consumer record** → `{repo, dependency, version, used_packages[], used_symbols[]}`

A private library is a **first-class object indexed from both sides**: the repo that
defines it (exports) and every repo that imports it (usages, with the specific
symbols actually used and the version pinned).

## Storage

SQLite. Contract/library tables hang off an `index_id` (the unit of a single
indexed repo snapshot). Core tables: `api_specs`, `http_operations`, `api_schemas`,
`proto_files`, `proto_services`, `proto_rpcs`, `proto_messages`, `dependencies`
(with `is_private` flag and `ecosystem`), `private_library_exports`,
`private_library_usages`. Full DDL in `docs/design.md`.

## MCP surface

### Tools (MVP set)
Existing Graphify tools: `list_repos`, `resolve_repo`, `query_repo`,
`explain_symbol`, `get_file_context`.

New API/library tools:
- `list_apis` — list OpenAPI + protobuf contracts in a repo/branch
- `find_endpoint` — find HTTP endpoint by NL / path / method / operationId
- `explain_endpoint` — endpoint explained from OpenAPI **and** source (returns service_flow chain)
- `find_rpc` / `explain_rpc` — same for protobuf RPCs (explain includes `consumed_by`)
- `find_schema` — find an OpenAPI schema or proto message
- `find_private_library` — find an internal lib by name/module-path/purpose
- `find_library_consumers` — given a library, list consumer repos + versions + used symbols

Later (post-MVP): `compare_api_versions`, `impact_api_change`, `explain_private_library`.

### Resources (URI schemes)
```
repo://...            graph://...
openapi://org/repo/commit/<sha>/spec/<path>
openapi://org/repo/commit/<sha>/operation/<operationId>
openapi://org/repo/commit/<sha>/schema/<schemaName>
proto://org/repo/commit/<sha>/file/<path>
proto://org/repo/commit/<sha>/service/<package>/<service>
proto://org/repo/commit/<sha>/rpc/<package>/<service>/<rpc>
proto://org/repo/commit/<sha>/message/<package>/<message>
lib://<module-path>[/version/<version>][/package/<package-path>][/symbol/<symbol>]
```
Resources are commit-pinned (`/commit/<sha>/`), so contract lookups are reproducible
against a specific snapshot.

## Retrieval strategy (how tools should answer)

For an API question, search **contract files first**, then walk into code via the graph:

1. Classify the question: HTTP API / gRPC / schema / SDK / library usage.
2. Search OpenAPI specs, proto files, dependency files.
3. Match operationId / path / RPC name / message name / module path.
4. Link the contract to implementation through graph nodes.
5. Expand to handler → service → repository → client code.
6. Search consumer repos when the question crosses service boundaries.
7. Return a compact answer: contracts + implementation + consumers (+ risk where relevant).

A correct answer to "what breaks if I change `ReserveProductRequest`?" must check
*all* of: OpenAPI schemas, proto messages, HTTP operations and RPCs using them,
generated Go structs, handler/server impls, consumer client code, and tests —
then report affected contracts/implementations/consumers and a risk level.
