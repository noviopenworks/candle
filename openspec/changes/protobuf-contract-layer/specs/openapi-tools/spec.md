## MODIFIED Requirements

### Requirement: list_apis lists API contracts
The system SHALL provide `list_apis` returning the API contracts indexed for a repo, each carrying a `kind` discriminator. It SHALL return OpenAPI specs as `{kind:"openapi", …}` entries and protobuf files as `{kind:"protobuf", …}` entries, so additional contract kinds can be added without breaking the output shape.

#### Scenario: Indexed OpenAPI specs are listed
- **WHEN** `list_apis` is called for a repo with one indexed OpenAPI spec
- **THEN** it returns an entry `{kind:"openapi", name, version, path}` for that spec

#### Scenario: Indexed protobuf files are listed
- **WHEN** `list_apis` is called for a repo with one indexed protobuf file
- **THEN** it returns an entry `{kind:"protobuf", name, version, path}` for that file alongside any OpenAPI entries

### Requirement: find_schema locates schemas and messages
The system SHALL provide `find_schema` that returns OpenAPI schemas and protobuf messages matching a query by name, each carrying a `kind` discriminator.

#### Scenario: OpenAPI schema found by name
- **WHEN** `find_schema` is called with a query matching a component schema name
- **THEN** it returns `{kind:"openapi_schema", name, spec_path}` for that schema

#### Scenario: Proto message found by name
- **WHEN** `find_schema` is called with a query matching a proto message name
- **THEN** it returns `{kind:"proto_message", name, spec_path}` for that message alongside any OpenAPI schema matches
