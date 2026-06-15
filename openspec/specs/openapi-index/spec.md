# openapi-index Specification

## Purpose
TBD - created by archiving change openapi-contract-layer. Update Purpose after archive.
## Requirements
### Requirement: OpenAPI spec discovery via manifest
The system SHALL read the OpenAPI spec file paths for a repo from an explicit `openapi:` list in that repo's manifest entry. The system SHALL NOT auto-discover specs by globbing the filesystem.

#### Scenario: Specs listed in manifest are indexed
- **WHEN** a manifest entry declares `openapi: [api/openapi.yaml]` and the file exists
- **THEN** that spec is parsed and stored under the repo's `index_id`

#### Scenario: Repo without openapi list indexes no specs
- **WHEN** a manifest entry has no `openapi:` field
- **THEN** the repo indexes successfully with zero API specs and no error

### Requirement: Parse OpenAPI 3.x into storage
The system SHALL parse OpenAPI 3.0/3.1 documents, resolving `$ref` and flattening components, and persist normalized spec metadata, HTTP operations, and schemas in tables tied to the repo's `index_id`.

#### Scenario: Operations and schemas are normalized
- **WHEN** an OpenAPI 3.x spec with paths and component schemas is indexed
- **THEN** each `(path, method)` is stored as an http_operation with operation_id, summary, request/response schema names, security, and tags, and each component schema is stored as an api_schema

#### Scenario: Swagger 2.0 is skipped with a warning
- **WHEN** a referenced spec file declares `swagger: "2.0"`
- **THEN** it is skipped with a warning and does not abort indexing of other specs

#### Scenario: Malformed or missing spec is tolerated
- **WHEN** a referenced spec file is missing or fails to parse
- **THEN** it is skipped with a warning and the rest of the run continues

### Requirement: Idempotent spec indexing
The system SHALL make spec indexing idempotent per `index_id`: re-indexing replaces that repo's API spec/operation/schema rows without duplication.

#### Scenario: Re-indexing the same repo does not duplicate
- **WHEN** a repo with one spec is indexed twice
- **THEN** the api_specs / http_operations / api_schemas counts are identical after the second run

