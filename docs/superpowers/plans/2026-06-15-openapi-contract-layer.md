---
change: openapi-contract-layer
design-doc: docs/superpowers/specs/2026-06-15-openapi-contract-layer-design.md
base-ref: 30924f5f411e9f9a9d427296ec1f35c19a143554
archived-with: 2026-06-15-openapi-contract-layer
---

# OpenAPI Contract Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Parse OpenAPI 3.x specs into SQLite (tied to the foundation's `index_id`) and serve them via four MCP tools and `openapi://` resources. Pure contract serving — no handler linking.

**Architecture:** Builds on the archived `mcp-core-foundation` (module `github.com/noviopenworks/candle`). Adds: `internal/config` manifest field `OpenAPI`, new `api_specs`/`http_operations`/`api_schemas` tables in `internal/store`, an `internal/openapi` parser (`kin-openapi`), store CRUD + queries, an ingest step, four pure tool functions, and `openapi://` resources registered in the existing MCP server.

**Tech Stack:** Go, `modernc.org/sqlite`, `github.com/getkin/kin-openapi/openapi3`, official MCP Go SDK, cobra/viper (all already wired by the foundation).

**Conventions:** TDD per task (failing test → run-fail → implement → run-pass → commit). `go test ./...`. Nullable TEXT columns read with `COALESCE(col,'')`. Tools are pure functions over the store (no SDK types); SDK confined to `server.go`.

archived-with: 2026-06-15-openapi-contract-layer
---

### Task 1: Manifest — add OpenAPI spec paths

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/testdata/manifest.yaml`

- [x] **Step 1: Extend fixture + write failing test**

Add to `internal/config/testdata/manifest.yaml` under the first repo:

```yaml
    openapi:
      - api/openapi.yaml
```

Append to `internal/config/config_test.go`:

```go
func TestOpenAPIPaths(t *testing.T) {
	cfg, err := Load("testdata/manifest.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Repos[0].OpenAPI) != 1 || cfg.Repos[0].OpenAPI[0] != "api/openapi.yaml" {
		t.Fatalf("expected one openapi path, got %+v", cfg.Repos[0].OpenAPI)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestOpenAPIPaths`
Expected: FAIL — `OpenAPI` field undefined.

- [x] **Step 3: Add the field**

In `internal/config/config.go`, add to `RepoConfig`:

```go
	OpenAPI []string `mapstructure:"openapi"`
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat(config): add openapi spec paths to manifest"
```

archived-with: 2026-06-15-openapi-contract-layer
---

### Task 2: Storage — api_specs / http_operations / api_schemas tables

**Files:**
- Modify: `internal/store/schema.go`
- Modify: `internal/store/store_test.go`

- [x] **Step 1: Write failing test**

Append to `internal/store/store_test.go`:

```go
func TestAPITablesExist(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	for _, tbl := range []string{"api_specs", "http_operations", "api_schemas"} {
		var name string
		if err := s.DB.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name); err != nil {
			t.Fatalf("expected table %q: %v", tbl, err)
		}
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestAPITablesExist`
Expected: FAIL — tables not found.

- [x] **Step 3: Append tables to `schemaSQL`**

Add to the `schemaSQL` const in `internal/store/schema.go` (before the closing backtick):

```sql
CREATE TABLE IF NOT EXISTS api_specs (
  id        INTEGER PRIMARY KEY,
  index_id  INTEGER NOT NULL REFERENCES indexes(id),
  kind      TEXT NOT NULL,
  name      TEXT,
  version   TEXT,
  path      TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS http_operations (
  id              INTEGER PRIMARY KEY,
  api_spec_id     INTEGER NOT NULL REFERENCES api_specs(id),
  method          TEXT NOT NULL,
  path            TEXT NOT NULL,
  operation_id    TEXT,
  summary         TEXT,
  request_schema  TEXT,
  response_schema TEXT,
  security        TEXT,
  tags            TEXT
);
CREATE TABLE IF NOT EXISTS api_schemas (
  id          INTEGER PRIMARY KEY,
  api_spec_id INTEGER NOT NULL REFERENCES api_specs(id),
  name        TEXT NOT NULL,
  kind        TEXT NOT NULL,
  raw_ref     TEXT
);
CREATE INDEX IF NOT EXISTS idx_http_ops_spec    ON http_operations(api_spec_id);
CREATE INDEX IF NOT EXISTS idx_http_ops_opid    ON http_operations(operation_id);
CREATE INDEX IF NOT EXISTS idx_api_schemas_spec ON api_schemas(api_spec_id);
CREATE INDEX IF NOT EXISTS idx_api_specs_index  ON api_specs(index_id);
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat(store): api_specs/http_operations/api_schemas tables"
```

archived-with: 2026-06-15-openapi-contract-layer
---

### Task 3: OpenAPI parser (`internal/openapi`)

**Files:**
- Create: `internal/openapi/openapi.go`
- Create: `internal/openapi/openapi_test.go`
- Create: `internal/openapi/testdata/inventory.yaml`
- Create: `internal/openapi/testdata/swagger2.yaml`

- [x] **Step 1: Write fixtures + failing test**

Create `internal/openapi/testdata/inventory.yaml`:

```yaml
openapi: 3.0.3
info:
  title: Inventory API
  version: 1.4.0
paths:
  /products/{productId}/reservations:
    post:
      operationId: reserveProduct
      summary: Reserve product stock
      tags: [reservations]
      security:
        - bearerAuth: []
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ReserveProductRequest'
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ReservationResponse'
components:
  schemas:
    ReserveProductRequest:
      type: object
      properties:
        productId: { type: string }
    ReservationResponse:
      type: object
      properties:
        status: { type: string }
```

Create `internal/openapi/testdata/swagger2.yaml`:

```yaml
swagger: "2.0"
info:
  title: Legacy
  version: "1.0"
paths: {}
```

Create `internal/openapi/openapi_test.go`:

```go
package openapi

import "testing"

func TestParseSpec(t *testing.T) {
	spec, err := ParseFile("testdata/inventory.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Name != "Inventory API" || spec.Version != "1.4.0" {
		t.Fatalf("bad meta: %+v", spec)
	}
	if len(spec.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(spec.Operations))
	}
	op := spec.Operations[0]
	if op.Method != "POST" || op.Path != "/products/{productId}/reservations" || op.OperationID != "reserveProduct" {
		t.Fatalf("bad op: %+v", op)
	}
	if op.RequestSchema != "ReserveProductRequest" || op.ResponseSchema != "ReservationResponse" {
		t.Fatalf("bad schemas: %+v", op)
	}
	if len(op.Security) != 1 || op.Security[0] != "bearerAuth" {
		t.Fatalf("bad security: %+v", op.Security)
	}
	if len(spec.Schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(spec.Schemas))
	}
}

func TestParseSwagger2IsRejected(t *testing.T) {
	_, err := ParseFile("testdata/swagger2.yaml")
	if err != ErrUnsupportedVersion {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/openapi/`
Expected: FAIL — `undefined: ParseFile`.

- [x] **Step 3: Implement the parser**

Create `internal/openapi/openapi.go`:

```go
package openapi

import (
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// ErrUnsupportedVersion is returned for Swagger 2.0 (or otherwise unsupported) docs.
var ErrUnsupportedVersion = errors.New("unsupported OpenAPI version (only 3.x)")

// Operation is a normalized HTTP operation.
type Operation struct {
	Method         string
	Path           string
	OperationID    string
	Summary        string
	RequestSchema  string
	ResponseSchema string
	Security       []string
	Tags           []string
}

// Schema is a normalized component schema.
type Schema struct {
	Name   string
	RawRef string
}

// Spec is a normalized OpenAPI document.
type Spec struct {
	Name       string
	Version    string
	Operations []Operation
	Schemas    []Schema
}

// ParseFile parses an OpenAPI 3.x document at path.
func ParseFile(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Reject Swagger 2.0 before handing to the 3.x loader.
	if strings.Contains(string(data), `swagger: "2.0"`) || strings.Contains(string(data), `"swagger":"2.0"`) {
		return nil, ErrUnsupportedVersion
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, err
	}
	if err := doc.Validate(loader.Context); err != nil {
		return nil, err
	}
	return normalize(doc), nil
}

func refName(ref string) string {
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

func normalize(doc *openapi3.T) *Spec {
	s := &Spec{}
	if doc.Info != nil {
		s.Name = doc.Info.Title
		s.Version = doc.Info.Version
	}
	for _, path := range sortedKeys(doc.Paths.Map()) {
		item := doc.Paths.Value(path)
		for method, op := range item.Operations() {
			no := Operation{
				Method:      method,
				Path:        path,
				OperationID: op.OperationID,
				Summary:     op.Summary,
				Tags:        op.Tags,
			}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				if mt := op.RequestBody.Value.Content.Get("application/json"); mt != nil && mt.Schema != nil && mt.Schema.Ref != "" {
					no.RequestSchema = refName(mt.Schema.Ref)
				}
			}
			for _, code := range sortedRespCodes(op.Responses) {
				resp := op.Responses.Value(code)
				if resp.Value == nil {
					continue
				}
				if mt := resp.Value.Content.Get("application/json"); mt != nil && mt.Schema != nil && mt.Schema.Ref != "" {
					no.ResponseSchema = refName(mt.Schema.Ref)
					break
				}
			}
			for _, req := range securityNames(op.Security) {
				no.Security = append(no.Security, req)
			}
			s.Operations = append(s.Operations, no)
		}
	}
	if doc.Components != nil {
		for _, name := range sortedKeys(mapToSet(doc.Components.Schemas)) {
			s.Schemas = append(s.Schemas, Schema{Name: name, RawRef: "#/components/schemas/" + name})
		}
	}
	return s
}

func securityNames(reqs *openapi3.SecurityRequirements) []string {
	var out []string
	if reqs == nil {
		return out
	}
	for _, req := range *reqs {
		for name := range req {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func mapToSet(m openapi3.Schemas) map[string]struct{} {
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

func sortedRespCodes(r *openapi3.Responses) []string {
	if r == nil {
		return nil
	}
	return sortedKeys(r.Map())
}
```

> Note: `kin-openapi`'s exact accessor names (`Paths.Map()`, `Responses.Map()`, `Content.Get`) are version-dependent. Verify against the resolved module version at implementation time and adjust these accessors only; the `Spec`/`Operation`/`Schema` shapes stay fixed.

- [x] **Step 4: Run test to verify it passes**

Run: `go get github.com/getkin/kin-openapi/openapi3 && go mod tidy && go test ./internal/openapi/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/openapi go.mod go.sum
git commit -m "feat(openapi): kin-openapi 3.x parser with swagger2 rejection"
```

archived-with: 2026-06-15-openapi-contract-layer
---

### Task 4: Store — api CRUD + queries

**Files:**
- Create: `internal/store/api.go`
- Create: `internal/store/api_test.go`

- [x] **Step 1: Write failing test**

Create `internal/store/api_test.go`:

```go
package store

import "testing"

func seedAPI(t *testing.T) (*Store, int64) {
	t.Helper()
	s, _ := Open(":memory:")
	id, _ := s.UpsertIndex("org", "svc", "abc", "main", "/g")
	spec := APISpec{Kind: "openapi", Name: "Inventory API", Version: "1.4.0", Path: "api/openapi.yaml"}
	ops := []HTTPOperation{{Method: "POST", Path: "/x", OperationID: "reserveProduct", Summary: "Reserve",
		RequestSchema: "ReserveProductRequest", ResponseSchema: "ReservationResponse", Security: []string{"bearerAuth"}, Tags: []string{"reservations"}}}
	schemas := []APISchema{{Name: "ReserveProductRequest", Kind: "openapi_schema", RawRef: "#/components/schemas/ReserveProductRequest"}}
	if err := s.ReplaceAPISpecs(id, []APISpecBundle{{Spec: spec, Operations: ops, Schemas: schemas}}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	return s, id
}

func TestListAPISpecsAndIdempotent(t *testing.T) {
	s, id := seedAPI(t)
	defer s.Close()
	specs, err := s.ListAPISpecs(id)
	if err != nil || len(specs) != 1 || specs[0].Name != "Inventory API" {
		t.Fatalf("list: %+v err=%v", specs, err)
	}
	// Replace again → still 1 (idempotent)
	s.ReplaceAPISpecs(id, []APISpecBundle{{Spec: specs[0].APISpec, Operations: nil, Schemas: nil}})
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM api_specs WHERE index_id=?`, id).Scan(&n)
	if n != 1 {
		t.Fatalf("expected 1 spec after replace, got %d", n)
	}
}

func TestFindOperationAndSchema(t *testing.T) {
	s, id := seedAPI(t)
	defer s.Close()
	ops, err := s.FindOperations(id, "reserveProduct")
	if err != nil || len(ops) != 1 || ops[0].OperationID != "reserveProduct" {
		t.Fatalf("find op: %+v err=%v", ops, err)
	}
	op, found, err := s.OperationByMethodPath(id, "POST", "/x")
	if err != nil || !found || op.ResponseSchema != "ReservationResponse" {
		t.Fatalf("op by method/path: %+v found=%v err=%v", op, found, err)
	}
	sc, err := s.FindSchemas(id, "Reserve")
	if err != nil || len(sc) != 1 {
		t.Fatalf("find schema: %+v err=%v", sc, err)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run 'API|Operation|Schema'`
Expected: FAIL — `undefined: APISpec`.

- [x] **Step 3: Implement**

Create `internal/store/api.go`:

```go
package store

import (
	"encoding/json"
	"strings"
)

// APISpec is a stored API contract's metadata.
type APISpec struct {
	ID      int64
	Kind    string
	Name    string
	Version string
	Path    string
}

// APISpecRow is an APISpec with its index_id.
type APISpecRow struct {
	APISpec
	IndexID int64
}

// HTTPOperation is a stored HTTP operation.
type HTTPOperation struct {
	Method         string
	Path           string
	OperationID    string
	Summary        string
	RequestSchema  string
	ResponseSchema string
	Security       []string
	Tags           []string
	SpecPath       string
}

// APISchema is a stored schema.
type APISchema struct {
	Name     string
	Kind     string
	RawRef   string
	SpecPath string
}

// APISpecBundle groups a spec with its operations and schemas for insertion.
type APISpecBundle struct {
	Spec       APISpec
	Operations []HTTPOperation
	Schemas    []APISchema
}

// ReplaceAPISpecs replaces all API specs (and their operations/schemas) for indexID.
func (s *Store) ReplaceAPISpecs(indexID int64, bundles []APISpecBundle) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Delete children then specs for this index.
	if _, err := tx.Exec(`DELETE FROM http_operations WHERE api_spec_id IN (SELECT id FROM api_specs WHERE index_id=?)`, indexID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM api_schemas WHERE api_spec_id IN (SELECT id FROM api_specs WHERE index_id=?)`, indexID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM api_specs WHERE index_id=?`, indexID); err != nil {
		return err
	}
	for _, b := range bundles {
		res, err := tx.Exec(`INSERT INTO api_specs(index_id, kind, name, version, path) VALUES(?,?,?,?,?)`,
			indexID, b.Spec.Kind, b.Spec.Name, b.Spec.Version, b.Spec.Path)
		if err != nil {
			return err
		}
		specID, _ := res.LastInsertId()
		for _, op := range b.Operations {
			if _, err := tx.Exec(
				`INSERT INTO http_operations(api_spec_id, method, path, operation_id, summary, request_schema, response_schema, security, tags)
				 VALUES(?,?,?,?,?,?,?,?,?)`,
				specID, op.Method, op.Path, op.OperationID, op.Summary, op.RequestSchema, op.ResponseSchema,
				jsonList(op.Security), jsonList(op.Tags)); err != nil {
				return err
			}
		}
		for _, sc := range b.Schemas {
			if _, err := tx.Exec(`INSERT INTO api_schemas(api_spec_id, name, kind, raw_ref) VALUES(?,?,?,?)`,
				specID, sc.Name, sc.Kind, sc.RawRef); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func jsonList(v []string) string {
	if len(v) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func parseList(s string) []string {
	var out []string
	if s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

// ListAPISpecs returns specs for indexID.
func (s *Store) ListAPISpecs(indexID int64) ([]APISpecRow, error) {
	rows, err := s.DB.Query(`SELECT id, index_id, kind, COALESCE(name,''), COALESCE(version,''), path FROM api_specs WHERE index_id=?`, indexID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APISpecRow
	for rows.Next() {
		var r APISpecRow
		if err := rows.Scan(&r.ID, &r.IndexID, &r.Kind, &r.Name, &r.Version, &r.Path); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

const opCols = `o.method, o.path, COALESCE(o.operation_id,''), COALESCE(o.summary,''),
  COALESCE(o.request_schema,''), COALESCE(o.response_schema,''), COALESCE(o.security,''), COALESCE(o.tags,''), s.path`

func scanOps(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]HTTPOperation, error) {
	defer rows.Close()
	var out []HTTPOperation
	for rows.Next() {
		var o HTTPOperation
		var sec, tags string
		if err := rows.Scan(&o.Method, &o.Path, &o.OperationID, &o.Summary, &o.RequestSchema, &o.ResponseSchema, &sec, &tags, &o.SpecPath); err != nil {
			return nil, err
		}
		o.Security, o.Tags = parseList(sec), parseList(tags)
		out = append(out, o)
	}
	return out, rows.Err()
}

// FindOperations matches operations in indexID by operationId/path/method substring (case-insensitive).
func (s *Store) FindOperations(indexID int64, query string) ([]HTTPOperation, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`SELECT `+opCols+`
		FROM http_operations o JOIN api_specs s ON s.id=o.api_spec_id
		WHERE s.index_id=? AND (
		  LOWER(COALESCE(o.operation_id,'')) LIKE ? OR LOWER(o.path) LIKE ? OR LOWER(o.method) LIKE ? OR LOWER(COALESCE(o.summary,'')) LIKE ?)`,
		indexID, q, q, q, q)
	if err != nil {
		return nil, err
	}
	return scanOps(rows)
}

// OperationByMethodPath returns the operation matching method+path exactly.
func (s *Store) OperationByMethodPath(indexID int64, method, path string) (HTTPOperation, bool, error) {
	rows, err := s.DB.Query(`SELECT `+opCols+`
		FROM http_operations o JOIN api_specs s ON s.id=o.api_spec_id
		WHERE s.index_id=? AND UPPER(o.method)=UPPER(?) AND o.path=?`, indexID, method, path)
	if err != nil {
		return HTTPOperation{}, false, err
	}
	ops, err := scanOps(rows)
	if err != nil || len(ops) == 0 {
		return HTTPOperation{}, false, err
	}
	return ops[0], true, nil
}

// OperationByID returns the operation matching an operationId exactly.
func (s *Store) OperationByID(indexID int64, opID string) (HTTPOperation, bool, error) {
	rows, err := s.DB.Query(`SELECT `+opCols+`
		FROM http_operations o JOIN api_specs s ON s.id=o.api_spec_id
		WHERE s.index_id=? AND o.operation_id=?`, indexID, opID)
	if err != nil {
		return HTTPOperation{}, false, err
	}
	ops, err := scanOps(rows)
	if err != nil || len(ops) == 0 {
		return HTTPOperation{}, false, err
	}
	return ops[0], true, nil
}

// FindSchemas matches schemas in indexID by name substring (case-insensitive).
func (s *Store) FindSchemas(indexID int64, query string) ([]APISchema, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`SELECT sc.name, sc.kind, COALESCE(sc.raw_ref,''), s.path
		FROM api_schemas sc JOIN api_specs s ON s.id=sc.api_spec_id
		WHERE s.index_id=? AND LOWER(sc.name) LIKE ?`, indexID, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APISchema
	for rows.Next() {
		var sc APISchema
		if err := rows.Scan(&sc.Name, &sc.Kind, &sc.RawRef, &sc.SpecPath); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat(store): api spec/operation/schema CRUD + queries"
```

archived-with: 2026-06-15-openapi-contract-layer
---

### Task 5: Ingest — wire OpenAPI parsing into the index flow

**Files:**
- Modify: `internal/ingest/ingest.go`
- Modify: `internal/ingest/ingest_test.go`

- [x] **Step 1: Write failing test**

Append to `internal/ingest/ingest_test.go`:

```go
func TestRunIndexesOpenAPI(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	os.WriteFile(graphPath, []byte(`{"nodes":[],"edges":[]}`), 0o644)
	specPath := filepath.Join(dir, "openapi.yaml")
	os.WriteFile(specPath, []byte("openapi: 3.0.3\ninfo:\n  title: T\n  version: \"1\"\npaths:\n  /x:\n    get:\n      operationId: getX\n      responses:\n        '200': { description: ok }\n"), 0o644)

	cfg := &config.Config{Repos: []config.RepoConfig{
		{Repo: "org/svc", Graph: graphPath, Commit: "c1", OpenAPI: []string{specPath}},
	}}
	s, _ := store.Open(":memory:")
	defer s.Close()
	if _, err := Run(s, cfg); err != nil {
		t.Fatalf("run: %v", err)
	}
	var ops int
	s.DB.QueryRow(`SELECT COUNT(*) FROM http_operations`).Scan(&ops)
	if ops != 1 {
		t.Fatalf("expected 1 operation indexed, got %d", ops)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ingest/ -run TestRunIndexesOpenAPI`
Expected: FAIL — specs not indexed (0 operations).

- [x] **Step 3: Implement — add spec indexing after graph load**

In `internal/ingest/ingest.go`, after the successful `graph.Load(...)` for a repo, resolve specs and store them. Add imports `github.com/noviopenworks/candle/internal/openapi` and `path/filepath`, and after `rep.Indexed++` insert:

```go
		// OpenAPI specs (pure contract serving).
		var bundles []store.APISpecBundle
		for _, sp := range r.OpenAPI {
			spec, perr := openapi.ParseFile(sp)
			if perr != nil {
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: openapi %s: %v", r.Repo, sp, perr))
				continue
			}
			bundles = append(bundles, toBundle(spec, sp))
		}
		if err := s.ReplaceAPISpecs(indexID, bundles); err != nil {
			return rep, err
		}
```

Add the converter at the end of the file:

```go
func toBundle(spec *openapi.Spec, specPath string) store.APISpecBundle {
	b := store.APISpecBundle{Spec: store.APISpec{Kind: "openapi", Name: spec.Name, Version: spec.Version, Path: specPath}}
	for _, op := range spec.Operations {
		b.Operations = append(b.Operations, store.HTTPOperation{
			Method: op.Method, Path: op.Path, OperationID: op.OperationID, Summary: op.Summary,
			RequestSchema: op.RequestSchema, ResponseSchema: op.ResponseSchema, Security: op.Security, Tags: op.Tags,
		})
	}
	for _, sc := range spec.Schemas {
		b.Schemas = append(b.Schemas, store.APISchema{Name: sc.Name, Kind: "openapi_schema", RawRef: sc.RawRef})
	}
	return b
}
```

(`indexID` is already in scope from the `UpsertIndex` call earlier in the loop. Ensure `store` is imported.)

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ingest/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/ingest
git commit -m "feat(ingest): index openapi specs into the same index_id"
```

archived-with: 2026-06-15-openapi-contract-layer
---

### Task 6: Tools — list_apis / find_endpoint / explain_endpoint / find_schema

**Files:**
- Create: `internal/mcp/openapi_tools.go`
- Create: `internal/mcp/openapi_tools_test.go`

- [x] **Step 1: Write failing test**

Create `internal/mcp/openapi_tools_test.go`:

```go
package mcp

import (
	"testing"

	"github.com/noviopenworks/candle/internal/store"
)

func seedAPITools(t *testing.T) *Tools {
	t.Helper()
	s, _ := store.Open(":memory:")
	id, _ := s.UpsertIndex("org", "svc", "abc", "main", "/g")
	s.ReplaceAPISpecs(id, []store.APISpecBundle{{
		Spec:       store.APISpec{Kind: "openapi", Name: "Inventory API", Version: "1.4.0", Path: "api/openapi.yaml"},
		Operations: []store.HTTPOperation{{Method: "POST", Path: "/x", OperationID: "reserveProduct", ResponseSchema: "ReservationResponse"}},
		Schemas:    []store.APISchema{{Name: "ReservationResponse", Kind: "openapi_schema"}},
	}})
	return NewTools(s)
}

func TestListAPIs(t *testing.T) {
	tl := seedAPITools(t)
	apis, err := tl.ListAPIs("org/svc")
	if err != nil || len(apis) != 1 || apis[0].Kind != "openapi" || apis[0].Name != "Inventory API" {
		t.Fatalf("list_apis: %+v err=%v", apis, err)
	}
}

func TestExplainEndpoint(t *testing.T) {
	tl := seedAPITools(t)
	out, err := tl.ExplainEndpoint("org/svc", "POST", "/x")
	if err != nil {
		t.Fatal(err)
	}
	if out.OperationID != "reserveProduct" || out.ResponseSchema != "ReservationResponse" {
		t.Fatalf("explain: %+v", out)
	}
}

func TestExplainEndpointUnknown(t *testing.T) {
	tl := seedAPITools(t)
	if _, err := tl.ExplainEndpoint("org/svc", "GET", "/nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFindEndpointAndSchema(t *testing.T) {
	tl := seedAPITools(t)
	ops, err := tl.FindEndpoint("org/svc", "reserve")
	if err != nil || len(ops) != 1 {
		t.Fatalf("find_endpoint: %+v err=%v", ops, err)
	}
	sc, err := tl.FindSchema("org/svc", "Reservation")
	if err != nil || len(sc) != 1 {
		t.Fatalf("find_schema: %+v err=%v", sc, err)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run 'API|Endpoint|Schema'`
Expected: FAIL — `undefined: (*Tools).ListAPIs`.

- [x] **Step 3: Implement**

Create `internal/mcp/openapi_tools.go`:

```go
package mcp

import "github.com/noviopenworks/candle/internal/store"

// APIInfo is one entry in list_apis output (kind-discriminated for future contract kinds).
type APIInfo struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// ListAPIs implements list_apis for a repo.
func (t *Tools) ListAPIs(repo string) ([]APIInfo, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	specs, err := t.s.ListAPISpecs(ri.IndexID)
	if err != nil {
		return nil, err
	}
	out := make([]APIInfo, 0, len(specs))
	for _, sp := range specs {
		out = append(out, APIInfo{Kind: sp.Kind, Name: sp.Name, Version: sp.Version, Path: sp.Path})
	}
	return out, nil
}

// FindEndpoint implements find_endpoint (lexical match).
func (t *Tools) FindEndpoint(repo, query string) ([]store.HTTPOperation, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return t.s.FindOperations(ri.IndexID, query)
}

// ExplainEndpoint implements explain_endpoint (contract data only — no handler/service_flow).
func (t *Tools) ExplainEndpoint(repo, method, path string) (store.HTTPOperation, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return store.HTTPOperation{}, err
	}
	if !ok {
		return store.HTTPOperation{}, ErrNotFound
	}
	op, found, err := t.s.OperationByMethodPath(ri.IndexID, method, path)
	if err != nil {
		return store.HTTPOperation{}, err
	}
	if !found {
		return store.HTTPOperation{}, ErrNotFound
	}
	return op, nil
}

// FindSchema implements find_schema (OpenAPI schemas).
func (t *Tools) FindSchema(repo, query string) ([]store.APISchema, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return t.s.FindSchemas(ri.IndexID, query)
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/mcp
git commit -m "feat(mcp): openapi tools (list_apis/find_endpoint/explain_endpoint/find_schema)"
```

archived-with: 2026-06-15-openapi-contract-layer
---

### Task 7: Register OpenAPI tools + `openapi://` resources in the server

**Files:**
- Modify: `internal/mcp/server.go`

> **SDK note:** reuse the exact `AddTool` / `AddResourceTemplate` patterns already established in `server.go` for the base tools. Confine all SDK types to this file. Append the four new tool names to `ToolNames`.

- [x] **Step 1: Register tools**

In `NewServer`, after the base tool registrations, add `AddTool` calls for `list_apis`, `find_endpoint`, `explain_endpoint`, `find_schema` with typed arg structs:
- `list_apis`: `{Repo string}` → `tools.ListAPIs`
- `find_endpoint`: `{Repo, Query string}` → `tools.FindEndpoint`
- `explain_endpoint`: `{Repo, Method, Path string}` → `tools.ExplainEndpoint`
- `find_schema`: `{Repo, Query string}` → `tools.FindSchema`

Each handler mirrors the base-tool handlers: call the pure method, on `ErrNotFound` return an `IsError`/empty result (not a protocol error), else marshal the result to text content. Append the four names to `ToolNames`.

- [x] **Step 2: Register `openapi://` resources**

Add an `AddResourceTemplate` for `openapi://{repo}/commit/{sha}/...` (mirror the `graph://` handler). Parse the URI suffix to dispatch:
- `.../operation/<operationId>` → `tools` resolve repo → `store.OperationByID` → JSON
- `.../schema/<name>` → `store.FindSchemas` (exact-name filter) → JSON
- `.../spec/<path>` → `store.ListAPISpecs` filtered by path + its operations → JSON

Add small helpers `parseOpenAPIURI` and reuse `resourceText`/`toolErr`. Keep the resolution logic delegating to pure `Tools`/`store` methods.

- [x] **Step 3: Build + run full suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: build succeeds, vet clean, all unit tests PASS.

- [x] **Step 4: Commit**

```bash
git add internal/mcp
git commit -m "feat(mcp): register openapi tools + openapi:// resources"
```

archived-with: 2026-06-15-openapi-contract-layer
---

### Task 8: E2E + degradation sweep + tasks.md sync

**Files:**
- Modify: `internal/mcp/e2e_test.go`
- Modify: `openspec/changes/openapi-contract-layer/tasks.md`

- [x] **Step 1: Extend the E2E test**

Extend the existing subprocess E2E test (or add a sibling) to: write a fixture repo with an `openapi.yaml`, a manifest with `openapi:`, run `index`, serve, then assert via the SDK client that `tools/list` includes `list_apis`/`find_endpoint`/`explain_endpoint`/`find_schema`, and that `explain_endpoint {repo, "POST", "/x"}` returns the operationId.

- [x] **Step 2: Run E2E**

Run: `go test ./internal/mcp/ -run TestEndToEndStdio -v` (the test is named `TestEndToEndStdio`; `-run E2E` matches nothing)
Expected: PASS (asserts the four new tools + an explain_endpoint result).

- [x] **Step 3: Degradation checks**

Confirm tests cover: missing spec file (warn+skip, run continues), Swagger 2.0 (`ErrUnsupportedVersion` → warn+skip in ingest), unknown repo/endpoint/schema (`ErrNotFound`/empty). Add any missing as failing-first tests in `internal/ingest` / `internal/mcp`.

Run: `go test ./...`
Expected: PASS

- [x] **Step 4: Check off OpenSpec tasks.md**

Mark items 1.1–6.4 in `openspec/changes/openapi-contract-layer/tasks.md` as complete (`- [ ]` → `- [x]`).

- [x] **Step 5: Commit**

```bash
git add openspec/changes/openapi-contract-layer/tasks.md internal/mcp
git commit -m "test(mcp): e2e + degradation for openapi layer; check off tasks"
```

archived-with: 2026-06-15-openapi-contract-layer
---

## Self-Review

**Spec coverage:**
- `openapi-index` / manifest discovery → Task 1, Task 5. Parse 3.x + skip Swagger 2.0 + tolerate malformed → Task 3, Task 5, Task 8. Idempotent indexing → Task 4 (`ReplaceAPISpecs`), Task 5.
- `openapi-tools` / list_apis → Task 6. find_endpoint → Task 6. explain_endpoint (contract only) → Task 6. find_schema → Task 6. openapi:// resources → Task 7.

**Placeholder scan:** The only version-dependent specifics are kin-openapi accessor names (Task 3, flagged) and the SDK registration calls (Task 7, reuse established `server.go` patterns) — both gated with notes and confined to one file each. All store/tool/ingest code is complete and compilable.

**Type consistency:** `store.APISpec`/`APISpecRow`/`HTTPOperation`/`APISchema`/`APISpecBundle` are used consistently across Tasks 4, 5, 6. `Tools` methods (`ListAPIs`, `FindEndpoint`, `ExplainEndpoint`, `FindSchema`) consistent between Tasks 6 and 7. `openapi.Spec`/`Operation`/`Schema` consistent between Tasks 3 and 5. `ErrNotFound` reused from the foundation.
