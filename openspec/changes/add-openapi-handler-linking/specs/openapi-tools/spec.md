## MODIFIED Requirements

### Requirement: explain_endpoint returns contract data
The system SHALL provide `explain_endpoint` that returns the OpenAPI contract data for a `(method, path)`: summary, operationId, request schema, response schema, security, tags, and spec path. It SHALL additionally return an `implemented_by` field listing the handler symbol(s) linked to the operation by the AST linker, each carrying a confidence tier; when no handler link exists, `implemented_by` SHALL be an empty list (not an error). Existing contract fields SHALL be unchanged, so the addition is backward compatible.

#### Scenario: Contract data returned
- **WHEN** `explain_endpoint` is called with a known method and path
- **THEN** it returns the summary, operation_id, request_schema, response_schema, security, tags, and spec_path

#### Scenario: Handler link returned when implementation is indexed
- **WHEN** `explain_endpoint` is called for an operation whose handler was AST-linked during indexing
- **THEN** the result includes a non-empty `implemented_by` list naming the handler node(s) with a confidence tier

#### Scenario: Empty handler link when no implementation is linked
- **WHEN** `explain_endpoint` is called for an operation with no linked handler
- **THEN** the result includes an empty `implemented_by` list and the contract data is still returned

#### Scenario: Unknown endpoint returns not-found
- **WHEN** `explain_endpoint` is called with a method/path that is not indexed
- **THEN** it returns a structured not-found result, not an error/crash
