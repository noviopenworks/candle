## ADDED Requirements

### Requirement: list_apis lists OpenAPI contracts
The system SHALL provide `list_apis` returning the OpenAPI contracts indexed for a repo, each carrying a `kind` discriminator so other contract kinds (e.g. protobuf) can be added without breaking the output shape.

#### Scenario: Indexed specs are listed
- **WHEN** `list_apis` is called for a repo with one indexed OpenAPI spec
- **THEN** it returns an entry `{kind:"openapi", name, version, path}` for that spec

### Requirement: find_endpoint locates operations
The system SHALL provide `find_endpoint` that matches indexed operations by natural language, path, HTTP method, or operationId.

#### Scenario: Match by operationId
- **WHEN** `find_endpoint` is called with a query equal to a known operationId
- **THEN** the matching operation (method, path, operation_id, spec_path) is returned

#### Scenario: Match by path or method
- **WHEN** `find_endpoint` is called with a path fragment or method
- **THEN** operations whose path or method match are returned

### Requirement: explain_endpoint returns contract data
The system SHALL provide `explain_endpoint` that returns the OpenAPI contract data for a `(method, path)`: summary, operationId, request schema, response schema, security, tags, and spec path. It SHALL NOT return handler implementation or a service call flow in this change (linking is deferred).

#### Scenario: Contract data returned
- **WHEN** `explain_endpoint` is called with a known method and path
- **THEN** it returns the summary, operation_id, request_schema, response_schema, security, tags, and spec_path

#### Scenario: Unknown endpoint returns not-found
- **WHEN** `explain_endpoint` is called with a method/path that is not indexed
- **THEN** it returns a structured not-found result, not an error/crash

### Requirement: find_schema locates OpenAPI schemas
The system SHALL provide `find_schema` that returns OpenAPI schemas matching a query by name.

#### Scenario: Schema found by name
- **WHEN** `find_schema` is called with a query matching a component schema name
- **THEN** it returns `{kind:"openapi_schema", name, spec_path}` for that schema

### Requirement: OpenAPI resources
The system SHALL expose `openapi://` resources for a spec, an operation, and a schema, commit-pinned from manifest metadata and degrading to branch or latest when no commit is recorded.

#### Scenario: Operation resource returns the operation
- **WHEN** a client reads `openapi://org/name/commit/<sha>/operation/<operationId>` for an indexed operation
- **THEN** it returns that operation's contract data
