---
change: protobuf-contract-layer
design-doc: docs/superpowers/specs/2026-06-16-protobuf-contract-layer-design.md
base-ref: 9973fcb38d6a2d6ef5f36abab3244a1c80f85a84
archived-with: 2026-06-16-protobuf-contract-layer
---

# Protobuf Contract Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Parse `.proto` contracts into storage, link each RPC to its gRPC server implementation within the same repo, and serve the result through MCP tools and `proto://` resources.

**Architecture:** Mirror the shipped OpenAPI layer. A new `internal/proto` package compiles `.proto` files with `bufbuild/protocompile` into dedicated `index_id`-scoped SQLite tables. A new shared `internal/link` package matches RPCs to gRPC server-method code nodes (already loaded by `graph.Load`) with a confidence model. New `internal/mcp` tools (`find_rpc`, `explain_rpc`) and `proto://` resources expose the data; `list_apis`/`find_schema` gain additive protobuf entries.

**Tech Stack:** Go 1.26, modernc.org/sqlite, bufbuild/protocompile, modelcontextprotocol/go-sdk, spf13/viper.

archived-with: 2026-06-16-protobuf-contract-layer
---

## Conventions (read before starting)

- Module path: `github.com/noviopenworks/candle`.
- Store tests open `store.Open(":memory:")`; follow `internal/store/api_test.go`.
- Pure tool logic lives in `internal/mcp/*_tools.go`; SDK registration lives in `internal/mcp/server.go`.
- `stream_kind` is one of exactly: `unary`, `server_stream`, `client_stream`, `bidi`.
- Run the full suite with: `go test ./...` (expected: `ok` for every package).
- Commit after every task. Check off the matching `openspec/changes/protobuf-contract-layer/tasks.md` item (`- [ ]` → `- [x]`) in the same commit.

## File Structure

- Create `internal/store/proto.go` — proto types, bundles, `ReplaceProtoFiles`, `LinkRPCImpls`, and query methods.
- Modify `internal/store/schema.go` — add the six proto tables + indexes.
- Create `internal/store/proto_test.go` — storage + idempotency tests.
- Modify `internal/config/config.go` — add the `proto:` block to `RepoConfig`.
- Modify `internal/config/config_test.go` — assert proto config parses.
- Create `internal/proto/proto.go` — protocompile parser → normalized structs.
- Create `internal/proto/proto_test.go` — parser tests over fixture `.proto` files.
- Create `internal/proto/testdata/` — fixture `.proto` files.
- Create `internal/link/link.go` — RPC→impl matcher with confidence tiers.
- Create `internal/link/link_test.go` — linker tests.
- Modify `internal/ingest/ingest.go` — parse protos + run linker after `graph.Load`.
- Modify `internal/ingest/ingest_test.go` — proto ingest coverage.
- Create `internal/mcp/proto_tools.go` — `FindRPC`, `ExplainRPC`; extend `ListAPIs`/`FindSchema`.
- Create `internal/mcp/proto_tools_test.go` — tool tests.
- Modify `internal/mcp/resources.go` — `proto://` resource handlers.
- Modify `internal/mcp/server.go` — register tools/resources; extend `ToolNames`.
- Modify `internal/mcp/resources_test.go` / `e2e_test.go` — resource + regression coverage.

archived-with: 2026-06-16-protobuf-contract-layer
---

## Task 1: Storage schema and proto tables

**Files:**
- Modify: `internal/store/schema.go`
- Create: `internal/store/proto.go`
- Test: `internal/store/proto_test.go`

- [x] **Step 1: Add proto tables to the schema**

In `internal/store/schema.go`, append the following inside the `schemaSQL` backtick string, before the closing `` ` ``:

```sql
CREATE TABLE IF NOT EXISTS proto_files (
  id         INTEGER PRIMARY KEY,
  index_id   INTEGER NOT NULL REFERENCES indexes(id),
  path       TEXT NOT NULL,
  package    TEXT,
  go_package TEXT,
  imports    TEXT
);
CREATE TABLE IF NOT EXISTS proto_services (
  id            INTEGER PRIMARY KEY,
  proto_file_id INTEGER NOT NULL REFERENCES proto_files(id),
  name          TEXT NOT NULL,
  full_name     TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS proto_rpcs (
  id               INTEGER PRIMARY KEY,
  proto_service_id INTEGER NOT NULL REFERENCES proto_services(id),
  name             TEXT NOT NULL,
  full_name        TEXT NOT NULL,
  request_message  TEXT,
  response_message TEXT,
  stream_kind      TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS proto_messages (
  id            INTEGER PRIMARY KEY,
  proto_file_id INTEGER NOT NULL REFERENCES proto_files(id),
  name          TEXT NOT NULL,
  full_name     TEXT NOT NULL,
  fields        TEXT
);
CREATE TABLE IF NOT EXISTS proto_enums (
  id            INTEGER PRIMARY KEY,
  proto_file_id INTEGER NOT NULL REFERENCES proto_files(id),
  name          TEXT NOT NULL,
  full_name     TEXT NOT NULL,
  values        TEXT
);
CREATE TABLE IF NOT EXISTS proto_rpc_impls (
  id           INTEGER PRIMARY KEY,
  proto_rpc_id INTEGER NOT NULL REFERENCES proto_rpcs(id),
  node_id      TEXT NOT NULL,
  confidence   REAL NOT NULL,
  match_reason TEXT
);
CREATE INDEX IF NOT EXISTS idx_proto_files_index ON proto_files(index_id);
CREATE INDEX IF NOT EXISTS idx_proto_services_file ON proto_services(proto_file_id);
CREATE INDEX IF NOT EXISTS idx_proto_rpcs_service ON proto_rpcs(proto_service_id);
CREATE INDEX IF NOT EXISTS idx_proto_messages_file ON proto_messages(proto_file_id);
CREATE INDEX IF NOT EXISTS idx_proto_enums_file ON proto_enums(proto_file_id);
CREATE INDEX IF NOT EXISTS idx_proto_rpc_impls_rpc ON proto_rpc_impls(proto_rpc_id);
```

- [x] **Step 2: Write the failing storage test**

Create `internal/store/proto_test.go`:

```go
package store

import "testing"

func seedProto(t *testing.T) (*Store, int64) {
	t.Helper()
	s, _ := Open(":memory:")
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	bundle := ProtoFileBundle{
		File: ProtoFile{Path: "proto/inventory.proto", Package: "acme.inventory",
			GoPackage: "github.com/acme/inventory/gen", Imports: []string{"google/protobuf/timestamp.proto"}},
		Services: []ProtoServiceBundle{{
			Service: ProtoService{Name: "InventoryService", FullName: "acme.inventory.InventoryService"},
			RPCs: []ProtoRPC{{
				Name: "ReserveProduct", FullName: "acme.inventory.InventoryService.ReserveProduct",
				RequestMessage: "acme.inventory.ReserveProductRequest",
				ResponseMessage: "acme.inventory.ReserveProductResponse", StreamKind: "unary"}},
		}},
		Messages: []ProtoMessage{{Name: "ReserveProductRequest", FullName: "acme.inventory.ReserveProductRequest",
			Fields: []ProtoField{{Name: "sku", Type: "string", Number: 1, Label: "optional"}}}},
		Enums: []ProtoEnum{{Name: "Status", FullName: "acme.inventory.Status",
			Values: []ProtoEnumValue{{Name: "OK", Number: 0}}}},
	}
	if err := s.ReplaceProtoFiles(id, []ProtoFileBundle{bundle}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	return s, id
}

func TestProtoStorageAndIdempotent(t *testing.T) {
	s, id := seedProto(t)
	defer s.Close()

	files, err := s.ListProtoFiles(id)
	if err != nil || len(files) != 1 || files[0].Package != "acme.inventory" {
		t.Fatalf("list files: %+v err=%v", files, err)
	}
	rpcs, err := s.FindRPCs(id, "reserve", "")
	if err != nil || len(rpcs) != 1 || rpcs[0].StreamKind != "unary" || rpcs[0].ProtoPath != "proto/inventory.proto" {
		t.Fatalf("find rpcs: %+v err=%v", rpcs, err)
	}
	if got, err := s.FindRPCs(id, "reserve", "bidi"); err != nil || len(got) != 0 {
		t.Fatalf("stream filter: %+v err=%v", got, err)
	}
	msgs, err := s.FindMessages(id, "Reserve")
	if err != nil || len(msgs) != 1 || len(msgs[0].Fields) != 1 {
		t.Fatalf("find messages: %+v err=%v", msgs, err)
	}

	// Re-index → counts identical (idempotent).
	if err := s.ReplaceProtoFiles(id, []ProtoFileBundle{}); err != nil {
		t.Fatalf("re-replace: %v", err)
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM proto_files WHERE index_id=?`, id).Scan(&n)
	if n != 0 {
		t.Fatalf("expected 0 files after empty replace, got %d", n)
	}
}
```

- [x] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/store/ -run TestProtoStorage -v`
Expected: FAIL — `ProtoFileBundle`, `ReplaceProtoFiles`, etc. undefined.

- [x] **Step 4: Implement `internal/store/proto.go`**

Create `internal/store/proto.go`:

```go
package store

import "strings"

// ProtoFile is a stored .proto file's metadata.
type ProtoFile struct {
	ID        int64
	Path      string
	Package   string
	GoPackage string
	Imports   []string
}

// ProtoFileRow is a ProtoFile with its index_id.
type ProtoFileRow struct {
	ProtoFile
	IndexID int64
}

// ProtoService is a stored gRPC service.
type ProtoService struct {
	ID       int64
	Name     string
	FullName string
}

// ProtoRPC is a stored RPC.
type ProtoRPC struct {
	Name            string
	FullName        string
	RequestMessage  string
	ResponseMessage string
	StreamKind      string
}

// ProtoRPCResult is an RPC joined with its service and file path.
type ProtoRPCResult struct {
	ProtoRPC
	Service   string
	ProtoPath string
}

// ProtoField is one message field.
type ProtoField struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Number int32  `json:"number"`
	Label  string `json:"label"`
}

// ProtoMessage is a stored message.
type ProtoMessage struct {
	Name     string
	FullName string
	Fields   []ProtoField
}

// ProtoMessageResult is a message joined with its file path.
type ProtoMessageResult struct {
	ProtoMessage
	ProtoPath string
}

// ProtoEnumValue is one enum value.
type ProtoEnumValue struct {
	Name   string `json:"name"`
	Number int32  `json:"number"`
}

// ProtoEnum is a stored enum.
type ProtoEnum struct {
	Name     string
	FullName string
	Values   []ProtoEnumValue
}

// ProtoServiceBundle groups a service with its RPCs.
type ProtoServiceBundle struct {
	Service ProtoService
	RPCs    []ProtoRPC
}

// ProtoFileBundle groups a file with its services, messages, and enums.
type ProtoFileBundle struct {
	File     ProtoFile
	Services []ProtoServiceBundle
	Messages []ProtoMessage
	Enums    []ProtoEnum
}

// ProtoRPCImpl is a same-repo implementation link for an RPC.
type ProtoRPCImpl struct {
	NodeID      string
	Confidence  float64
	MatchReason string
}

// RPCImplLink is an impl link keyed by RPC full name, written by the linker.
type RPCImplLink struct {
	RPCFullName string
	NodeID      string
	Confidence  float64
	MatchReason string
}

func jsonBlob(v any) string { return mustMarshal(v) }

func mustMarshal(v any) string {
	b, _ := jsonMarshal(v)
	return string(b)
}

// ReplaceProtoFiles replaces all proto data (files/services/rpcs/messages/enums
// and impl links) for indexID. Idempotent.
func (s *Store) ReplaceProtoFiles(indexID int64, bundles []ProtoFileBundle) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Delete children first (impls → rpcs → services/messages/enums → files).
	stmts := []string{
		`DELETE FROM proto_rpc_impls WHERE proto_rpc_id IN (SELECT r.id FROM proto_rpcs r
		   JOIN proto_services sv ON sv.id=r.proto_service_id
		   JOIN proto_files f ON f.id=sv.proto_file_id WHERE f.index_id=?)`,
		`DELETE FROM proto_rpcs WHERE proto_service_id IN (SELECT sv.id FROM proto_services sv
		   JOIN proto_files f ON f.id=sv.proto_file_id WHERE f.index_id=?)`,
		`DELETE FROM proto_services WHERE proto_file_id IN (SELECT id FROM proto_files WHERE index_id=?)`,
		`DELETE FROM proto_messages WHERE proto_file_id IN (SELECT id FROM proto_files WHERE index_id=?)`,
		`DELETE FROM proto_enums WHERE proto_file_id IN (SELECT id FROM proto_files WHERE index_id=?)`,
		`DELETE FROM proto_files WHERE index_id=?`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q, indexID); err != nil {
			return err
		}
	}
	for _, b := range bundles {
		res, err := tx.Exec(`INSERT INTO proto_files(index_id, path, package, go_package, imports) VALUES(?,?,?,?,?)`,
			indexID, b.File.Path, b.File.Package, b.File.GoPackage, jsonList(b.File.Imports))
		if err != nil {
			return err
		}
		fileID, _ := res.LastInsertId()
		for _, sb := range b.Services {
			sres, err := tx.Exec(`INSERT INTO proto_services(proto_file_id, name, full_name) VALUES(?,?,?)`,
				fileID, sb.Service.Name, sb.Service.FullName)
			if err != nil {
				return err
			}
			svcID, _ := sres.LastInsertId()
			for _, r := range sb.RPCs {
				if _, err := tx.Exec(`INSERT INTO proto_rpcs(proto_service_id, name, full_name, request_message, response_message, stream_kind)
					VALUES(?,?,?,?,?,?)`, svcID, r.Name, r.FullName, r.RequestMessage, r.ResponseMessage, r.StreamKind); err != nil {
					return err
				}
			}
		}
		for _, m := range b.Messages {
			if _, err := tx.Exec(`INSERT INTO proto_messages(proto_file_id, name, full_name, fields) VALUES(?,?,?,?)`,
				fileID, m.Name, m.FullName, jsonBlob(m.Fields)); err != nil {
				return err
			}
		}
		for _, e := range b.Enums {
			if _, err := tx.Exec(`INSERT INTO proto_enums(proto_file_id, name, full_name, values) VALUES(?,?,?,?)`,
				fileID, e.Name, e.FullName, jsonBlob(e.Values)); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// LinkRPCImpls replaces all impl links for indexID, resolving RPC full names to
// proto_rpcs rows in this index. Unmatched RPC names are ignored.
func (s *Store) LinkRPCImpls(indexID int64, links []RPCImplLink) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM proto_rpc_impls WHERE proto_rpc_id IN (SELECT r.id FROM proto_rpcs r
		JOIN proto_services sv ON sv.id=r.proto_service_id
		JOIN proto_files f ON f.id=sv.proto_file_id WHERE f.index_id=?)`, indexID); err != nil {
		return err
	}
	for _, l := range links {
		if _, err := tx.Exec(`INSERT INTO proto_rpc_impls(proto_rpc_id, node_id, confidence, match_reason)
			SELECT r.id, ?, ?, ? FROM proto_rpcs r
			  JOIN proto_services sv ON sv.id=r.proto_service_id
			  JOIN proto_files f ON f.id=sv.proto_file_id
			WHERE f.index_id=? AND r.full_name=?`,
			l.NodeID, l.Confidence, l.MatchReason, indexID, l.RPCFullName); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListProtoFiles returns proto files for indexID.
func (s *Store) ListProtoFiles(indexID int64) ([]ProtoFileRow, error) {
	rows, err := s.DB.Query(`SELECT id, index_id, path, COALESCE(package,''), COALESCE(go_package,''), COALESCE(imports,'')
		FROM proto_files WHERE index_id=?`, indexID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProtoFileRow
	for rows.Next() {
		var r ProtoFileRow
		var imports string
		if err := rows.Scan(&r.ID, &r.IndexID, &r.Path, &r.Package, &r.GoPackage, &imports); err != nil {
			return nil, err
		}
		r.Imports = parseList(imports)
		out = append(out, r)
	}
	return out, rows.Err()
}

const rpcCols = `r.name, r.full_name, COALESCE(r.request_message,''), COALESCE(r.response_message,''),
  r.stream_kind, sv.name, f.path`

func scanRPCs(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]ProtoRPCResult, error) {
	defer rows.Close()
	var out []ProtoRPCResult
	for rows.Next() {
		var r ProtoRPCResult
		if err := rows.Scan(&r.Name, &r.FullName, &r.RequestMessage, &r.ResponseMessage, &r.StreamKind, &r.Service, &r.ProtoPath); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FindRPCs matches RPCs in indexID by name/service/full_name substring,
// optionally filtered to a stream_kind (empty = any).
func (s *Store) FindRPCs(indexID int64, query, streamKind string) ([]ProtoRPCResult, error) {
	q := "%" + strings.ToLower(query) + "%"
	sql := `SELECT ` + rpcCols + ` FROM proto_rpcs r
		JOIN proto_services sv ON sv.id=r.proto_service_id
		JOIN proto_files f ON f.id=sv.proto_file_id
		WHERE f.index_id=? AND (LOWER(r.name) LIKE ? OR LOWER(r.full_name) LIKE ? OR LOWER(sv.name) LIKE ?)`
	args := []any{indexID, q, q, q}
	if streamKind != "" {
		sql += ` AND r.stream_kind=?`
		args = append(args, streamKind)
	}
	rows, err := s.DB.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	return scanRPCs(rows)
}

// RPCByServiceName returns the RPC matching service + rpc name exactly.
func (s *Store) RPCByServiceName(indexID int64, service, rpc string) (ProtoRPCResult, bool, error) {
	rows, err := s.DB.Query(`SELECT `+rpcCols+` FROM proto_rpcs r
		JOIN proto_services sv ON sv.id=r.proto_service_id
		JOIN proto_files f ON f.id=sv.proto_file_id
		WHERE f.index_id=? AND sv.name=? AND r.name=?`, indexID, service, rpc)
	if err != nil {
		return ProtoRPCResult{}, false, err
	}
	got, err := scanRPCs(rows)
	if err != nil || len(got) == 0 {
		return ProtoRPCResult{}, false, err
	}
	return got[0], true, nil
}

// FindMessages matches messages in indexID by name substring.
func (s *Store) FindMessages(indexID int64, query string) ([]ProtoMessageResult, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`SELECT m.name, m.full_name, COALESCE(m.fields,''), f.path
		FROM proto_messages m JOIN proto_files f ON f.id=m.proto_file_id
		WHERE f.index_id=? AND (LOWER(m.name) LIKE ? OR LOWER(m.full_name) LIKE ?)`, indexID, q, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProtoMessageResult
	for rows.Next() {
		var m ProtoMessageResult
		var fields string
		if err := rows.Scan(&m.Name, &m.FullName, &fields, &m.ProtoPath); err != nil {
			return nil, err
		}
		m.Fields = parseFields(fields)
		out = append(out, m)
	}
	return out, rows.Err()
}

// ProtoRPCImpls returns impl links for an RPC full name in indexID.
func (s *Store) ProtoRPCImpls(indexID int64, rpcFullName string) ([]ProtoRPCImpl, error) {
	rows, err := s.DB.Query(`SELECT i.node_id, i.confidence, COALESCE(i.match_reason,'')
		FROM proto_rpc_impls i JOIN proto_rpcs r ON r.id=i.proto_rpc_id
		  JOIN proto_services sv ON sv.id=r.proto_service_id
		  JOIN proto_files f ON f.id=sv.proto_file_id
		WHERE f.index_id=? AND r.full_name=?`, indexID, rpcFullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProtoRPCImpl
	for rows.Next() {
		var im ProtoRPCImpl
		if err := rows.Scan(&im.NodeID, &im.Confidence, &im.MatchReason); err != nil {
			return nil, err
		}
		out = append(out, im)
	}
	return out, rows.Err()
}
```

Add the JSON helpers at the bottom of `internal/store/api.go` (next to `jsonList`/`parseList`, which already exist there):

```go
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func parseFields(s string) []ProtoField {
	var out []ProtoField
	if s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}
```

- [x] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/store/ -v`
Expected: PASS (including existing API tests).

- [x] **Step 6: Commit**

```bash
git add internal/store/schema.go internal/store/proto.go internal/store/api.go internal/store/proto_test.go openspec/changes/protobuf-contract-layer/tasks.md
git commit -m "feat(store): proto contract tables and queries"
```
Then mark tasks.md items 1.1 and 1.2 as `[x]`.

archived-with: 2026-06-16-protobuf-contract-layer
---

## Task 2: Manifest proto config

**Files:**
- Modify: `internal/config/config.go:11-18`
- Test: `internal/config/config_test.go`

- [x] **Step 1: Write the failing config test**

Add to `internal/config/config_test.go`:

```go
func TestProtoConfigParses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	yaml := "repos:\n" +
		"  - repo: acme/inventory\n" +
		"    graph: /tmp/g.json\n" +
		"    proto:\n" +
		"      roots: [proto]\n" +
		"      files: [proto/inventory.proto]\n"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	r := cfg.Repos[0]
	if len(r.Proto.Roots) != 1 || r.Proto.Roots[0] != "proto" {
		t.Fatalf("roots: %+v", r.Proto.Roots)
	}
	if len(r.Proto.Files) != 1 || r.Proto.Files[0] != "proto/inventory.proto" {
		t.Fatalf("files: %+v", r.Proto.Files)
	}
}
```

Ensure the test file imports `os`, `path/filepath`, and `testing` (add any missing imports).

- [x] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/config/ -run TestProtoConfig -v`
Expected: FAIL — `r.Proto` undefined.

- [x] **Step 3: Add the proto block to `RepoConfig`**

In `internal/config/config.go`, modify the `RepoConfig` struct (currently ending after the `OpenAPI` field) to add:

```go
	Proto struct {
		Roots []string `mapstructure:"roots"`
		Files []string `mapstructure:"files"`
	} `mapstructure:"proto"`
```

Place it immediately after the `OpenAPI []string` field, inside the struct.

- [x] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go openspec/changes/protobuf-contract-layer/tasks.md
git commit -m "feat(config): proto roots/files manifest block"
```
Then mark tasks.md item 2.1 as `[x]`.

archived-with: 2026-06-16-protobuf-contract-layer
---

## Task 3: Protobuf parser (`internal/proto`)

**Files:**
- Create: `internal/proto/proto.go`
- Create: `internal/proto/proto_test.go`
- Create: `internal/proto/testdata/inventory.proto`
- Modify: `go.mod` / `go.sum` (add bufbuild/protocompile)

- [x] **Step 1: Add the protocompile dependency**

Run:
```bash
go get github.com/bufbuild/protocompile@latest
```
Expected: `go.mod` gains `github.com/bufbuild/protocompile`.

- [x] **Step 2: Create the fixture proto**

Create `internal/proto/testdata/inventory.proto`:

```proto
syntax = "proto3";
package acme.inventory;
option go_package = "github.com/acme/inventory/gen";

service InventoryService {
  rpc ReserveProduct(ReserveProductRequest) returns (ReserveProductResponse);
  rpc WatchStock(WatchStockRequest) returns (stream StockEvent);
  rpc UploadCounts(stream CountRecord) returns (UploadSummary);
  rpc Sync(stream SyncMsg) returns (stream SyncMsg);
}

message ReserveProductRequest {
  string sku = 1;
  int32 quantity = 2;
}
message ReserveProductResponse { bool reserved = 1; }
message WatchStockRequest { string sku = 1; }
message StockEvent { int32 level = 1; }
message CountRecord { string sku = 1; }
message UploadSummary { int32 total = 1; }
message SyncMsg { string payload = 1; }

enum Status {
  STATUS_UNKNOWN = 0;
  STATUS_OK = 1;
}
```

- [x] **Step 3: Write the failing parser test**

Create `internal/proto/proto_test.go`:

```go
package proto

import "testing"

func TestParseInventory(t *testing.T) {
	files, warns, err := ParseFiles([]string{"testdata"}, []string{"inventory.proto"})
	if err != nil {
		t.Fatalf("parse: %v (warns=%v)", err, warns)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Package != "acme.inventory" || f.GoPackage != "github.com/acme/inventory/gen" {
		t.Fatalf("file meta: %+v", f)
	}
	if len(f.Services) != 1 || len(f.Services[0].RPCs) != 4 {
		t.Fatalf("services: %+v", f.Services)
	}
	kinds := map[string]string{}
	for _, r := range f.Services[0].RPCs {
		kinds[r.Name] = r.StreamKind
	}
	want := map[string]string{
		"ReserveProduct": "unary", "WatchStock": "server_stream",
		"UploadCounts": "client_stream", "Sync": "bidi",
	}
	for name, sk := range want {
		if kinds[name] != sk {
			t.Fatalf("rpc %s stream_kind=%q want %q", name, kinds[name], sk)
		}
	}
	var reserve *RPC
	for i := range f.Services[0].RPCs {
		if f.Services[0].RPCs[i].Name == "ReserveProduct" {
			reserve = &f.Services[0].RPCs[i]
		}
	}
	if reserve.RequestMessage != "acme.inventory.ReserveProductRequest" {
		t.Fatalf("request msg: %q", reserve.RequestMessage)
	}
	if reserve.FullName != "acme.inventory.InventoryService.ReserveProduct" {
		t.Fatalf("rpc full name: %q", reserve.FullName)
	}
	var req *Message
	for i := range f.Messages {
		if f.Messages[i].Name == "ReserveProductRequest" {
			req = &f.Messages[i]
		}
	}
	if req == nil || len(req.Fields) != 2 || req.Fields[0].Name != "sku" {
		t.Fatalf("message fields: %+v", req)
	}
	if len(f.Enums) != 1 || f.Enums[0].Name != "Status" || len(f.Enums[0].Values) != 2 {
		t.Fatalf("enums: %+v", f.Enums)
	}
}

func TestParseMissingFileWarns(t *testing.T) {
	files, warns, err := ParseFiles([]string{"testdata"}, []string{"nope.proto"})
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if len(files) != 0 || len(warns) == 0 {
		t.Fatalf("want 0 files + a warning, got files=%d warns=%v", len(files), warns)
	}
}
```

- [x] **Step 4: Run the test to verify it fails**

Run: `go test ./internal/proto/ -v`
Expected: FAIL — `ParseFiles`, `RPC`, `Message` undefined.

- [x] **Step 5: Implement `internal/proto/proto.go`**

Create `internal/proto/proto.go`:

```go
package proto

import (
	"context"
	"fmt"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// RPC is a normalized gRPC method.
type RPC struct {
	Name            string
	FullName        string
	RequestMessage  string
	ResponseMessage string
	StreamKind      string // unary|server_stream|client_stream|bidi
}

// Service is a normalized gRPC service.
type Service struct {
	Name     string
	FullName string
	RPCs     []RPC
}

// Field is a normalized message field.
type Field struct {
	Name   string
	Type   string
	Number int32
	Label  string
}

// Message is a normalized protobuf message.
type Message struct {
	Name     string
	FullName string
	Fields   []Field
}

// EnumValue is a normalized enum value.
type EnumValue struct {
	Name   string
	Number int32
}

// Enum is a normalized protobuf enum.
type Enum struct {
	Name     string
	FullName string
	Values   []EnumValue
}

// File is a normalized .proto file.
type File struct {
	Path      string
	Package   string
	GoPackage string
	Imports   []string
	Services  []Service
	Messages  []Message
	Enums     []Enum
}

// ParseFiles compiles importPaths-rooted .proto files. Each entry in files is an
// import-relative path. Returns normalized files, non-fatal warnings, and a hard
// error only on unexpected failures (a file that fails to compile is reported as
// a warning, not a hard error).
func ParseFiles(roots, files []string) ([]File, []string, error) {
	if len(files) == 0 {
		return nil, nil, nil
	}
	var warns []string
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{ImportPaths: roots}),
	}
	out := make([]File, 0, len(files))
	for _, rel := range files {
		fds, err := compiler.Compile(context.Background(), rel)
		if err != nil {
			warns = append(warns, fmt.Sprintf("%s: %v", rel, err))
			continue
		}
		for _, fd := range fds {
			out = append(out, normalizeFile(rel, fd))
		}
	}
	return out, warns, nil
}

func normalizeFile(path string, fd protoreflect.FileDescriptor) File {
	f := File{Path: path, Package: string(fd.Package())}
	if opts, ok := fd.Options().(*descriptorpb.FileDescriptorOptions); ok && opts != nil {
		f.GoPackage = opts.GetGoPackage()
	}
	imps := fd.Imports()
	for i := 0; i < imps.Len(); i++ {
		f.Imports = append(f.Imports, imps.Get(i).Path())
	}
	svcs := fd.Services()
	for i := 0; i < svcs.Len(); i++ {
		f.Services = append(f.Services, normalizeService(svcs.Get(i)))
	}
	msgs := fd.Messages()
	for i := 0; i < msgs.Len(); i++ {
		f.Messages = append(f.Messages, normalizeMessage(msgs.Get(i)))
	}
	ens := fd.Enums()
	for i := 0; i < ens.Len(); i++ {
		f.Enums = append(f.Enums, normalizeEnum(ens.Get(i)))
	}
	return f
}

func normalizeService(sd protoreflect.ServiceDescriptor) Service {
	s := Service{Name: string(sd.Name()), FullName: string(sd.FullName())}
	ms := sd.Methods()
	for i := 0; i < ms.Len(); i++ {
		m := ms.Get(i)
		s.RPCs = append(s.RPCs, RPC{
			Name:            string(m.Name()),
			FullName:        string(m.FullName()),
			RequestMessage:  string(m.Input().FullName()),
			ResponseMessage: string(m.Output().FullName()),
			StreamKind:      streamKind(m.IsStreamingClient(), m.IsStreamingServer()),
		})
	}
	return s
}

func streamKind(client, server bool) string {
	switch {
	case client && server:
		return "bidi"
	case client:
		return "client_stream"
	case server:
		return "server_stream"
	default:
		return "unary"
	}
}

func normalizeMessage(md protoreflect.MessageDescriptor) Message {
	m := Message{Name: string(md.Name()), FullName: string(md.FullName())}
	fs := md.Fields()
	for i := 0; i < fs.Len(); i++ {
		fd := fs.Get(i)
		m.Fields = append(m.Fields, Field{
			Name:   string(fd.Name()),
			Type:   fieldType(fd),
			Number: int32(fd.Number()),
			Label:  fieldLabel(fd),
		})
	}
	return m
}

func fieldType(fd protoreflect.FieldDescriptor) string {
	if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
		return string(fd.Message().FullName())
	}
	if fd.Kind() == protoreflect.EnumKind {
		return string(fd.Enum().FullName())
	}
	return fd.Kind().String()
}

func fieldLabel(fd protoreflect.FieldDescriptor) string {
	switch {
	case fd.IsList():
		return "repeated"
	case fd.IsMap():
		return "map"
	default:
		return "optional"
	}
}

func normalizeEnum(ed protoreflect.EnumDescriptor) Enum {
	e := Enum{Name: string(ed.Name()), FullName: string(ed.FullName())}
	vs := ed.Values()
	for i := 0; i < vs.Len(); i++ {
		v := vs.Get(i)
		e.Values = append(e.Values, EnumValue{Name: string(v.Name()), Number: int32(v.Number())})
	}
	return e
}
```

- [x] **Step 6: Tidy modules and run the test**

Run:
```bash
go mod tidy
go test ./internal/proto/ -v
```
Expected: PASS. (`google.golang.org/protobuf` is pulled in transitively by protocompile; `go mod tidy` resolves it.)

- [x] **Step 7: Commit**

```bash
git add internal/proto/ go.mod go.sum openspec/changes/protobuf-contract-layer/tasks.md
git commit -m "feat(proto): protocompile parser for services/rpcs/messages/enums"
```
Then mark tasks.md items 2.2, 2.3, 2.4 as `[x]`.

archived-with: 2026-06-16-protobuf-contract-layer
---

## Task 4: RPC→impl linker (`internal/link`)

**Files:**
- Create: `internal/link/link.go`
- Test: `internal/link/link_test.go`

The linker matches RPCs to gRPC server-method nodes in the same index. Signals: method-name match, service registration presence, and a best-effort source-signature streaming check.

- [x] **Step 1: Write the failing linker test**

Create `internal/link/link_test.go`:

```go
package link

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/noviopenworks/candle/internal/store"
)

func TestMatchRPCsConfidence(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "server.go")
	// Unary impl signature for ReserveProduct; bidi for Sync.
	code := "package svc\n" +
		"func (s *Server) ReserveProduct(ctx context.Context, req *pb.ReserveProductRequest) (*pb.ReserveProductResponse, error) { return nil, nil }\n" +
		"func (s *Server) Sync(stream pb.InventoryService_SyncServer) error { return nil }\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	// Seed graph nodes: the two impl methods + the generated registration symbol.
	mustNode(t, s, id, "n1", "ReserveProduct", src)
	mustNode(t, s, id, "n2", "Sync", src)
	mustNode(t, s, id, "n3", "RegisterInventoryServiceServer", src)

	rpcs := []RPC{
		{FullName: "acme.inventory.InventoryService.ReserveProduct", Service: "InventoryService", Name: "ReserveProduct", StreamKind: "unary"},
		{FullName: "acme.inventory.InventoryService.Sync", Service: "InventoryService", Name: "Sync", StreamKind: "bidi"},
		{FullName: "acme.inventory.InventoryService.Ghost", Service: "InventoryService", Name: "Ghost", StreamKind: "unary"},
	}
	links, err := MatchRPCs(s, id, rpcs)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	byRPC := map[string][]store.RPCImplLink{}
	for _, l := range links {
		byRPC[l.RPCFullName] = append(byRPC[l.RPCFullName], l)
	}
	rp := byRPC["acme.inventory.InventoryService.ReserveProduct"]
	if len(rp) != 1 || rp[0].NodeID != "n1" || rp[0].Confidence < 0.85 {
		t.Fatalf("ReserveProduct link: %+v", rp)
	}
	sy := byRPC["acme.inventory.InventoryService.Sync"]
	if len(sy) != 1 || sy[0].NodeID != "n2" || sy[0].Confidence < 0.85 {
		t.Fatalf("Sync link: %+v", sy)
	}
	if len(byRPC["acme.inventory.InventoryService.Ghost"]) != 0 {
		t.Fatalf("Ghost should have no impl")
	}
}

func mustNode(t *testing.T, s *store.Store, indexID int64, id, label, file string) {
	t.Helper()
	if _, err := s.DB.Exec(`INSERT INTO nodes(index_id, node_id, label, file_type, source_file) VALUES(?,?,?,?,?)`,
		indexID, id, label, "go", file); err != nil {
		t.Fatal(err)
	}
}
```

- [x] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/link/ -v`
Expected: FAIL — `RPC`, `MatchRPCs` undefined.

- [x] **Step 3: Implement `internal/link/link.go`**

Create `internal/link/link.go`:

```go
// Package link matches contract operations (currently proto RPCs) to their
// implementation symbols in a repo's code graph. It is intentionally generic so
// the OpenAPI handler linker can adopt it later.
package link

import (
	"os"
	"strings"

	"github.com/noviopenworks/candle/internal/store"
)

// RPC is the subset of an RPC the linker needs.
type RPC struct {
	FullName   string
	Service    string
	Name       string
	StreamKind string
}

const (
	confHigh   = 0.9
	confMedium = 0.6
	confLow    = 0.3
)

// MatchRPCs returns impl links for rpcs within a single index. Each candidate is
// recorded; ambiguous matches keep their tier rather than being dropped or
// collapsed.
func MatchRPCs(s *store.Store, indexID int64, rpcs []RPC) ([]store.RPCImplLink, error) {
	var out []store.RPCImplLink
	for _, r := range rpcs {
		nodes, err := s.NodesByLabel(indexID, r.Name)
		if err != nil {
			return nil, err
		}
		serviceRegistered, err := hasServiceRegistration(s, indexID, r.Service)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			conf, reason := score(n, r, serviceRegistered)
			out = append(out, store.RPCImplLink{
				RPCFullName: r.FullName, NodeID: n.NodeID, Confidence: conf, MatchReason: reason,
			})
		}
	}
	return out, nil
}

func hasServiceRegistration(s *store.Store, indexID int64, service string) (bool, error) {
	for _, label := range []string{"Register" + service + "Server", service + "Server"} {
		nodes, err := s.NodesByLabel(indexID, label)
		if err != nil {
			return false, err
		}
		if len(nodes) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func score(n store.NodeRow, r RPC, serviceRegistered bool) (float64, string) {
	reason := "name"
	conf := confLow
	if serviceRegistered {
		reason = "name+service"
		conf = confMedium
	}
	if signatureMatches(n.SourceFile, r.Name, r.StreamKind) {
		reason += "+signature"
		conf = confHigh
	}
	return conf, reason
}

// signatureMatches best-effort reads the impl source and checks the method's
// parameter shape against the RPC stream_kind. Unreadable source returns false
// (the caller keeps the lower-confidence name/service match).
func signatureMatches(sourceFile, rpcName, streamKind string) bool {
	if sourceFile == "" {
		return false
	}
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, ")") || !strings.Contains(line, rpcName+"(") {
			continue
		}
		if !strings.Contains(line, "func") {
			continue
		}
		streaming := strings.Contains(line, "Server)") || strings.Contains(line, "Server ")
		unary := strings.Contains(line, "context.Context") && !streaming
		switch streamKind {
		case "unary":
			return unary
		default:
			return streaming
		}
	}
	return false
}
```

- [x] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/link/ -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/link/ openspec/changes/protobuf-contract-layer/tasks.md
git commit -m "feat(link): shared RPC-to-impl linker with confidence tiers"
```
Then mark tasks.md items 3.1 and 3.3 as `[x]`.

archived-with: 2026-06-16-protobuf-contract-layer
---

## Task 5: Ingest wiring

**Files:**
- Modify: `internal/ingest/ingest.go`
- Test: `internal/ingest/ingest_test.go`

- [x] **Step 1: Write the failing ingest test**

Add to `internal/ingest/ingest_test.go` (reuse existing test helpers there for building a config + graph file; follow the existing test's setup style). The test must:

```go
func TestRunIndexesProtos(t *testing.T) {
	dir := t.TempDir()
	// Minimal graph.json with one node matching the RPC name.
	graphPath := filepath.Join(dir, "g.json")
	graphJSON := `{"nodes":[{"id":"n1","label":"ReserveProduct","source_file":"x.go"}],"edges":[],"hyperedges":[]}`
	if err := os.WriteFile(graphPath, []byte(graphJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Repos: []config.RepoConfig{{
		Repo: "acme/inventory", Graph: graphPath,
	}}}
	cfg.Repos[0].Proto.Roots = []string{protoTestdataDir()}
	cfg.Repos[0].Proto.Files = []string{"inventory.proto"}

	s, _ := store.Open(":memory:")
	defer s.Close()
	if _, err := Run(s, cfg); err != nil {
		t.Fatalf("run: %v", err)
	}
	id, _, _ := s.UpsertIndexLookup("acme", "inventory") // see note below
	rpcs, err := s.FindRPCs(id, "reserve", "")
	if err != nil || len(rpcs) != 1 {
		t.Fatalf("rpcs: %+v err=%v", rpcs, err)
	}
}
```

Note: if `UpsertIndexLookup` does not exist, resolve the index id via the existing registry/list path used elsewhere in `ingest_test.go` (e.g. `store.UpsertIndex` returns the id; call it with the same args Run used, which is idempotent). Use `filepath.Abs` to build `protoTestdataDir()` pointing at `../proto/testdata`. Keep this test aligned with the helpers already present in the file rather than inventing new store methods.

- [x] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/ingest/ -run TestRunIndexesProtos -v`
Expected: FAIL — protos not parsed yet.

- [x] **Step 3: Wire proto parsing + linking into `ingest.Run`**

In `internal/ingest/ingest.go`, add imports for `github.com/noviopenworks/candle/internal/proto` and `github.com/noviopenworks/candle/internal/link`. After the existing OpenAPI block (`s.ReplaceAPISpecs(indexID, bundles)`), add:

```go
		// Protobuf contracts.
		pfiles, pwarns, perr := proto.ParseFiles(r.Proto.Roots, r.Proto.Files)
		if perr != nil {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: proto: %v", r.Repo, perr))
		}
		for _, w := range pwarns {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: proto %s", r.Repo, w))
		}
		protoBundles := toProtoBundles(pfiles)
		if err := s.ReplaceProtoFiles(indexID, protoBundles); err != nil {
			return rep, err
		}
		links, err := link.MatchRPCs(s, indexID, collectRPCs(pfiles))
		if err != nil {
			return rep, err
		}
		if err := s.LinkRPCImpls(indexID, links); err != nil {
			return rep, err
		}
```

Add the two helpers at the bottom of the file:

```go
func toProtoBundles(files []proto.File) []store.ProtoFileBundle {
	var out []store.ProtoFileBundle
	for _, f := range files {
		b := store.ProtoFileBundle{File: store.ProtoFile{
			Path: f.Path, Package: f.Package, GoPackage: f.GoPackage, Imports: f.Imports}}
		for _, sv := range f.Services {
			sb := store.ProtoServiceBundle{Service: store.ProtoService{Name: sv.Name, FullName: sv.FullName}}
			for _, r := range sv.RPCs {
				sb.RPCs = append(sb.RPCs, store.ProtoRPC{
					Name: r.Name, FullName: r.FullName, RequestMessage: r.RequestMessage,
					ResponseMessage: r.ResponseMessage, StreamKind: r.StreamKind})
			}
			b.Services = append(b.Services, sb)
		}
		for _, m := range f.Messages {
			pm := store.ProtoMessage{Name: m.Name, FullName: m.FullName}
			for _, fld := range m.Fields {
				pm.Fields = append(pm.Fields, store.ProtoField{
					Name: fld.Name, Type: fld.Type, Number: fld.Number, Label: fld.Label})
			}
			b.Messages = append(b.Messages, pm)
		}
		for _, e := range f.Enums {
			pe := store.ProtoEnum{Name: e.Name, FullName: e.FullName}
			for _, v := range e.Values {
				pe.Values = append(pe.Values, store.ProtoEnumValue{Name: v.Name, Number: v.Number})
			}
			b.Enums = append(b.Enums, pe)
		}
		out = append(out, b)
	}
	return out
}

func collectRPCs(files []proto.File) []link.RPC {
	var out []link.RPC
	for _, f := range files {
		for _, sv := range f.Services {
			for _, r := range sv.RPCs {
				out = append(out, link.RPC{
					FullName: r.FullName, Service: sv.Name, Name: r.Name, StreamKind: r.StreamKind})
			}
		}
	}
	return out
}
```

- [x] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/ingest/ -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/ingest/ openspec/changes/protobuf-contract-layer/tasks.md
git commit -m "feat(ingest): parse and link protos after graph load"
```
Then mark tasks.md items 3.2 as `[x]`.

archived-with: 2026-06-16-protobuf-contract-layer
---

## Task 6: MCP tools (find_rpc, explain_rpc, list_apis/find_schema extensions)

**Files:**
- Create: `internal/mcp/proto_tools.go`
- Test: `internal/mcp/proto_tools_test.go`

- [x] **Step 1: Write the failing tool test**

Create `internal/mcp/proto_tools_test.go`:

```go
package mcp

import (
	"testing"

	"github.com/noviopenworks/candle/internal/store"
)

func seedProtoTools(t *testing.T) *Tools {
	t.Helper()
	s, _ := store.Open(":memory:")
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	bundle := store.ProtoFileBundle{
		File: store.ProtoFile{Path: "proto/inventory.proto", Package: "acme.inventory"},
		Services: []store.ProtoServiceBundle{{
			Service: store.ProtoService{Name: "InventoryService", FullName: "acme.inventory.InventoryService"},
			RPCs: []store.ProtoRPC{{Name: "ReserveProduct",
				FullName: "acme.inventory.InventoryService.ReserveProduct",
				RequestMessage: "acme.inventory.ReserveProductRequest",
				ResponseMessage: "acme.inventory.ReserveProductResponse", StreamKind: "unary"}}}},
		Messages: []store.ProtoMessage{{Name: "ReserveProductRequest", FullName: "acme.inventory.ReserveProductRequest",
			Fields: []store.ProtoField{{Name: "sku", Type: "string", Number: 1, Label: "optional"}}}},
	}
	if err := s.ReplaceProtoFiles(id, []store.ProtoFileBundle{bundle}); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkRPCImpls(id, []store.RPCImplLink{{
		RPCFullName: "acme.inventory.InventoryService.ReserveProduct",
		NodeID: "n1", Confidence: 0.9, MatchReason: "name+service+signature"}}); err != nil {
		t.Fatal(err)
	}
	return NewTools(s)
}

func TestFindRPCAndFilter(t *testing.T) {
	tools := seedProtoTools(t)
	got, err := tools.FindRPC("acme/inventory", "reserve", "")
	if err != nil || len(got) != 1 || got[0].StreamKind != "unary" {
		t.Fatalf("find: %+v err=%v", got, err)
	}
	if filtered, err := tools.FindRPC("acme/inventory", "reserve", "bidi"); err != nil || len(filtered) != 0 {
		t.Fatalf("filter: %+v err=%v", filtered, err)
	}
}

func TestExplainRPC(t *testing.T) {
	tools := seedProtoTools(t)
	out, err := tools.ExplainRPC("acme/inventory", "InventoryService", "ReserveProduct")
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if out.RPC.StreamKind != "unary" || len(out.ImplementedBy) != 1 || out.ImplementedBy[0].NodeID != "n1" {
		t.Fatalf("explain shape: %+v", out)
	}
	if out.ConsumedBy == "" {
		t.Fatalf("consumed_by should be a deferred marker, got empty")
	}
	if len(out.RequestMessageFields) != 1 || out.RequestMessageFields[0].Name != "sku" {
		t.Fatalf("request fields: %+v", out.RequestMessageFields)
	}
	if _, err := tools.ExplainRPC("acme/inventory", "InventoryService", "Nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestListAPIsIncludesProto(t *testing.T) {
	tools := seedProtoTools(t)
	apis, err := tools.ListAPIs("acme/inventory")
	if err != nil {
		t.Fatal(err)
	}
	var sawProto bool
	for _, a := range apis {
		if a.Kind == "protobuf" && a.Path == "proto/inventory.proto" {
			sawProto = true
		}
	}
	if !sawProto {
		t.Fatalf("list_apis missing protobuf entry: %+v", apis)
	}
}

func TestFindSchemaIncludesProtoMessage(t *testing.T) {
	tools := seedProtoTools(t)
	out, err := tools.FindSchema("acme/inventory", "Reserve")
	if err != nil {
		t.Fatal(err)
	}
	var sawMsg bool
	for _, sc := range out {
		if sc.Kind == "proto_message" && sc.Name == "ReserveProductRequest" {
			sawMsg = true
		}
	}
	if !sawMsg {
		t.Fatalf("find_schema missing proto_message: %+v", out)
	}
}
```

This test assumes `FindSchema` returns a slice of a kind-discriminated struct that includes a `Kind` field for both openapi and proto entries. Step 3 changes `FindSchema`'s return type accordingly; update `internal/mcp/openapi_tools_test.go` if it asserts the old `[]store.APISchema` return.

- [x] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/mcp/ -run 'RPC|ListAPIsIncludesProto|FindSchemaIncludesProto' -v`
Expected: FAIL — `FindRPC`, `ExplainRPC`, proto entries undefined.

- [x] **Step 3: Implement `internal/mcp/proto_tools.go` and extend ListAPIs/FindSchema**

Create `internal/mcp/proto_tools.go`:

```go
package mcp

import "github.com/noviopenworks/candle/internal/store"

// consumedByDeferred is the explicit marker returned until cross-repo consumer
// linking ships in a later change.
const consumedByDeferred = "deferred: cross-repo consumed_by not available in this change"

// SchemaInfo is a kind-discriminated find_schema entry (openapi_schema|proto_message).
type SchemaInfo struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	SpecPath string `json:"spec_path"`
}

// RPCExplanation is the explain_rpc result.
type RPCExplanation struct {
	RPC                   store.ProtoRPCResult `json:"rpc"`
	RequestMessageFields  []store.ProtoField   `json:"request_message_fields"`
	ResponseMessageFields []store.ProtoField   `json:"response_message_fields"`
	ImplementedBy         []store.ProtoRPCImpl `json:"implemented_by"`
	Calls                 []store.EdgeRow      `json:"calls"`
	ConsumedBy            string               `json:"consumed_by"`
}

// FindRPC implements find_rpc (lexical match + optional stream_kind filter).
func (t *Tools) FindRPC(repo, query, streamKind string) ([]store.ProtoRPCResult, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return t.s.FindRPCs(ri.IndexID, query, streamKind)
}

// ExplainRPC implements explain_rpc: proto facts + resolved messages + same-repo
// impls + best-effort one-hop calls + deferred consumed_by marker.
func (t *Tools) ExplainRPC(repo, service, rpc string) (RPCExplanation, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return RPCExplanation{}, err
	}
	if !ok {
		return RPCExplanation{}, ErrNotFound
	}
	r, found, err := t.s.RPCByServiceName(ri.IndexID, service, rpc)
	if err != nil {
		return RPCExplanation{}, err
	}
	if !found {
		return RPCExplanation{}, ErrNotFound
	}
	out := RPCExplanation{RPC: r, ConsumedBy: consumedByDeferred}
	out.RequestMessageFields = t.messageFields(ri.IndexID, r.RequestMessage)
	out.ResponseMessageFields = t.messageFields(ri.IndexID, r.ResponseMessage)
	impls, err := t.s.ProtoRPCImpls(ri.IndexID, r.FullName)
	if err != nil {
		return RPCExplanation{}, err
	}
	out.ImplementedBy = impls
	// Best-effort one-hop calls from the highest-confidence impl node.
	if best := bestImpl(impls); best != "" {
		calls, err := t.s.Callees(ri.IndexID, best)
		if err != nil {
			return RPCExplanation{}, err
		}
		out.Calls = calls
	}
	return out, nil
}

func (t *Tools) messageFields(indexID int64, fullName string) []store.ProtoField {
	if fullName == "" {
		return nil
	}
	msgs, err := t.s.FindMessages(indexID, fullName)
	if err != nil {
		return nil
	}
	for _, m := range msgs {
		if m.FullName == fullName {
			return m.Fields
		}
	}
	return nil
}

func bestImpl(impls []store.ProtoRPCImpl) string {
	var best string
	var top float64 = -1
	for _, im := range impls {
		if im.Confidence > top {
			top, best = im.Confidence, im.NodeID
		}
	}
	return best
}
```

Now extend `ListAPIs` in `internal/mcp/openapi_tools.go` to append protobuf entries. After the existing loop that appends OpenAPI specs (before `return out, nil`), add:

```go
	pfiles, err := t.s.ListProtoFiles(ri.IndexID)
	if err != nil {
		return nil, err
	}
	for _, pf := range pfiles {
		out = append(out, APIInfo{Kind: "protobuf", Name: pf.Package, Version: "", Path: pf.Path})
	}
```

Replace `FindSchema` in `internal/mcp/openapi_tools.go` with a kind-discriminated version:

```go
// FindSchema implements find_schema (OpenAPI schemas + proto messages).
func (t *Tools) FindSchema(repo, query string) ([]SchemaInfo, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	out := []SchemaInfo{}
	schemas, err := t.s.FindSchemas(ri.IndexID, query)
	if err != nil {
		return nil, err
	}
	for _, sc := range schemas {
		out = append(out, SchemaInfo{Kind: "openapi_schema", Name: sc.Name, SpecPath: sc.SpecPath})
	}
	msgs, err := t.s.FindMessages(ri.IndexID, query)
	if err != nil {
		return nil, err
	}
	for _, m := range msgs {
		out = append(out, SchemaInfo{Kind: "proto_message", Name: m.Name, SpecPath: m.ProtoPath})
	}
	return out, nil
}
```

Update `internal/mcp/resources.go` `SchemaResource` (which calls `FindSchemas`) to keep working — it uses `t.s.FindSchemas` directly (store method, unchanged), so no edit needed there. Update `internal/mcp/openapi_tools_test.go` if it asserts the old `FindSchema` return type (`[]store.APISchema`); change those assertions to the new `[]SchemaInfo` shape with `Kind == "openapi_schema"`.

- [x] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/mcp/ -v`
Expected: PASS (fix any `openapi_tools_test.go` assertions broken by the `FindSchema` return-type change).

- [x] **Step 5: Commit**

```bash
git add internal/mcp/proto_tools.go internal/mcp/openapi_tools.go internal/mcp/proto_tools_test.go internal/mcp/openapi_tools_test.go openspec/changes/protobuf-contract-layer/tasks.md
git commit -m "feat(mcp): find_rpc/explain_rpc and protobuf list_apis/find_schema"
```
Then mark tasks.md items 4.1, 4.2, 4.3, 4.4 as `[x]`.

archived-with: 2026-06-16-protobuf-contract-layer
---

## Task 7: MCP proto resources + server registration

**Files:**
- Modify: `internal/mcp/resources.go`
- Modify: `internal/mcp/server.go`
- Test: `internal/mcp/resources_test.go`

- [x] **Step 1: Write the failing resource test**

Add to `internal/mcp/resources_test.go` (follow the file's existing seeding style; reuse `seedProtoTools` from `proto_tools_test.go` since it's the same package):

```go
func TestProtoResources(t *testing.T) {
	tools := seedProtoTools(t)

	rpcBody, err := tools.ProtoRPCResource("acme/inventory", "acme.inventory", "InventoryService", "ReserveProduct")
	if err != nil || !strings.Contains(rpcBody, "ReserveProduct") {
		t.Fatalf("rpc resource: %q err=%v", rpcBody, err)
	}
	msgBody, err := tools.ProtoMessageResource("acme/inventory", "acme.inventory", "ReserveProductRequest")
	if err != nil || !strings.Contains(msgBody, "sku") {
		t.Fatalf("message resource: %q err=%v", msgBody, err)
	}
	if _, err := tools.ProtoMessageResource("acme/inventory", "acme.inventory", "Nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestParseProtoURI(t *testing.T) {
	repo, kind, ref, err := parseProtoURI("proto://acme/inventory/commit/abc/rpc/acme.inventory/InventoryService/ReserveProduct")
	if err != nil || repo != "acme/inventory" || kind != "rpc" || ref != "acme.inventory/InventoryService/ReserveProduct" {
		t.Fatalf("parse: repo=%q kind=%q ref=%q err=%v", repo, kind, ref, err)
	}
}
```

Add `"strings"` to the test imports if not present.

- [x] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/mcp/ -run 'ProtoResources|ParseProtoURI' -v`
Expected: FAIL — resource methods + parser undefined.

- [x] **Step 3: Implement the proto resources**

Add to `internal/mcp/resources.go`:

```go
// ProtoFileResource returns JSON for proto://…/file/<path>.
func (t *Tools) ProtoFileResource(repo, path string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	files, err := t.s.ListProtoFiles(ri.IndexID)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if f.Path == path {
			return mustJSON(f), nil
		}
	}
	return "", ErrNotFound
}

// ProtoRPCResource returns JSON for proto://…/rpc/<pkg>/<service>/<rpc>.
func (t *Tools) ProtoRPCResource(repo, pkg, service, rpc string) (string, error) {
	out, err := t.ExplainRPC(repo, service, rpc)
	if err != nil {
		return "", err
	}
	return mustJSON(out), nil
}

// ProtoServiceResource returns JSON for proto://…/service/<pkg>/<service>.
func (t *Tools) ProtoServiceResource(repo, pkg, service string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	rpcs, err := t.s.FindRPCs(ri.IndexID, "", "")
	if err != nil {
		return "", err
	}
	var matched []store.ProtoRPCResult
	for _, r := range rpcs {
		if r.Service == service {
			matched = append(matched, r)
		}
	}
	if len(matched) == 0 {
		return "", ErrNotFound
	}
	return mustJSON(map[string]any{"service": service, "package": pkg, "rpcs": matched}), nil
}

// ProtoMessageResource returns JSON for proto://…/message/<pkg>/<message>.
func (t *Tools) ProtoMessageResource(repo, pkg, message string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	msgs, err := t.s.FindMessages(ri.IndexID, message)
	if err != nil {
		return "", err
	}
	full := pkg + "." + message
	for _, m := range msgs {
		if m.Name == message || m.FullName == full || m.FullName == message {
			return mustJSON(m), nil
		}
	}
	return "", ErrNotFound
}
```

Add the URI parser to `internal/mcp/server.go` (next to `parseOpenAPIURI`):

```go
// parseProtoURI parses proto://org/name/commit/<sha>/<kind>/<ref...>.
func parseProtoURI(uri string) (repo, kind, ref string, err error) {
	rest := strings.TrimPrefix(uri, "proto://")
	parts := strings.Split(rest, "/")
	if len(parts) < 6 || parts[2] != "commit" {
		return "", "", "", fmt.Errorf("malformed proto uri %q", uri)
	}
	return parts[0] + "/" + parts[1], parts[4], strings.Join(parts[5:], "/"), nil
}
```

- [x] **Step 4: Register the proto tools and resources**

In `internal/mcp/server.go`:

1. Add the new tool names to `ToolNames` (after `"find_schema"`):

```go
	"find_rpc",
	"explain_rpc",
```

2. In `NewServer`, after `registerFindSchema(srv, tools)`, add:

```go
	registerFindRPC(srv, tools)
	registerExplainRPC(srv, tools)
```

3. Add the two tool registrations (follow the existing `register*` style):

```go
type findRPCArgs struct {
	Repo       string `json:"repo" jsonschema:"repo identity (org/name)"`
	Query      string `json:"query" jsonschema:"lexical match: NL / service / rpc / full name"`
	StreamKind string `json:"stream_kind" jsonschema:"optional filter: unary|server_stream|client_stream|bidi"`
}

func registerFindRPC(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "find_rpc",
		Description: "Find gRPC RPCs in a repo by lexical match, optionally filtered by stream_kind.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args findRPCArgs) (*mcpsdk.CallToolResult, any, error) {
		rpcs, err := tools.FindRPC(args.Repo, args.Query, args.StreamKind)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(rpcs)), nil, nil
	})
}

type explainRPCArgs struct {
	Repo    string `json:"repo" jsonschema:"repo identity (org/name)"`
	Service string `json:"service" jsonschema:"gRPC service name"`
	RPC     string `json:"rpc" jsonschema:"RPC method name"`
}

func registerExplainRPC(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "explain_rpc",
		Description: "Explain a gRPC RPC: proto facts, messages, same-repo implementation, one-hop calls.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args explainRPCArgs) (*mcpsdk.CallToolResult, any, error) {
		out, err := tools.ExplainRPC(args.Repo, args.Service, args.RPC)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(out)), nil, nil
	})
}
```

4. In `registerResources`, add a `proto` resource template after the `openapi` template:

```go
	srv.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		Name:        "proto",
		Description: "A protobuf artifact (file/service/rpc/message) as JSON, commit-pinned.",
		MIMEType:    "application/json",
		URITemplate: "proto://{org}/{name}/commit/{sha}/{kind}/{ref}",
	}, func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		repo, kind, ref, err := parseProtoURI(req.Params.URI)
		if err != nil {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		var body string
		switch kind {
		case "file":
			body, err = tools.ProtoFileResource(repo, ref)
		case "service":
			if p := strings.SplitN(ref, "/", 2); len(p) == 2 {
				body, err = tools.ProtoServiceResource(repo, p[0], p[1])
			} else {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
		case "rpc":
			if p := strings.SplitN(ref, "/", 3); len(p) == 3 {
				body, err = tools.ProtoRPCResource(repo, p[0], p[1], p[2])
			} else {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
		case "message":
			if p := strings.SplitN(ref, "/", 2); len(p) == 2 {
				body, err = tools.ProtoMessageResource(repo, p[0], p[1])
			} else {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
		default:
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		if err != nil {
			if err == ErrNotFound {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
			return nil, err
		}
		return resourceText(req.Params.URI, body), nil
	})
```

- [x] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/mcp/ -v`
Expected: PASS. If `e2e_test.go` asserts `ToolNames` length/content, update it to include `find_rpc` and `explain_rpc`.

- [x] **Step 6: Commit**

```bash
git add internal/mcp/resources.go internal/mcp/server.go internal/mcp/resources_test.go internal/mcp/e2e_test.go openspec/changes/protobuf-contract-layer/tasks.md
git commit -m "feat(mcp): proto:// resources and tool/resource registration"
```
Then mark tasks.md items 5.1, 5.2, 5.3, 5.4 as `[x]`.

archived-with: 2026-06-16-protobuf-contract-layer
---

## Task 8: Verification and regression

**Files:**
- Modify: `internal/mcp/e2e_test.go` (or the existing end-to-end test)

- [x] **Step 1: Add a regression assertion for unchanged HTTP output**

In `internal/mcp/e2e_test.go`, add a test that seeds a repo with BOTH an OpenAPI spec and a proto file, then asserts the OpenAPI-only results are unchanged:

```go
func TestProtoDoesNotRegressHTTP(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	// OpenAPI seed (mirror store/api_test seed).
	if err := s.ReplaceAPISpecs(id, []store.APISpecBundle{{
		Spec:    store.APISpec{Kind: "openapi", Name: "Inventory API", Version: "1.0", Path: "api/openapi.yaml"},
		Schemas: []store.APISchema{{Name: "ReserveProductRequest", Kind: "openapi_schema"}},
	}}); err != nil {
		t.Fatal(err)
	}
	// Proto seed.
	if err := s.ReplaceProtoFiles(id, []store.ProtoFileBundle{{
		File:     store.ProtoFile{Path: "proto/inventory.proto", Package: "acme.inventory"},
		Messages: []store.ProtoMessage{{Name: "ReserveProductRequest", FullName: "acme.inventory.ReserveProductRequest"}},
	}}); err != nil {
		t.Fatal(err)
	}
	tools := NewTools(s)

	apis, _ := tools.ListAPIs("acme/inventory")
	var openapiCount, protoCount int
	for _, a := range apis {
		switch a.Kind {
		case "openapi":
			openapiCount++
		case "protobuf":
			protoCount++
		}
	}
	if openapiCount != 1 || protoCount != 1 {
		t.Fatalf("list_apis kinds: openapi=%d proto=%d", openapiCount, protoCount)
	}

	out, _ := tools.FindSchema("acme/inventory", "ReserveProductRequest")
	var openapiSchema, protoMsg int
	for _, sc := range out {
		switch sc.Kind {
		case "openapi_schema":
			openapiSchema++
		case "proto_message":
			protoMsg++
		}
	}
	if openapiSchema != 1 || protoMsg != 1 {
		t.Fatalf("find_schema kinds: openapi=%d proto=%d", openapiSchema, protoMsg)
	}
}
```

- [x] **Step 2: Run the full suite**

Run: `go test ./...`
Expected: every package reports `ok`.

- [x] **Step 3: Build the binary**

Run: `go build ./...`
Expected: no output (success).

- [x] **Step 4: Commit**

```bash
git add internal/mcp/e2e_test.go openspec/changes/protobuf-contract-layer/tasks.md
git commit -m "test: proto/HTTP coexistence regression"
```
Then mark tasks.md items 6.1, 6.2, 6.3, 6.4, 6.5 as `[x]`.

archived-with: 2026-06-16-protobuf-contract-layer
---

## Self-Review Notes

- **Spec coverage:** protobuf-index (discovery → Task 2; parse/stream_kind → Task 3; same-repo linking → Task 4/5; uses_message via resolvable message refs → Task 6 `messageFields`; idempotency → Task 1/5). protobuf-tools (find_rpc + filter → Task 6; explain_rpc impl+calls+deferred consumed_by → Task 6; proto:// resources → Task 7). openapi-tools MODIFIED (list_apis/find_schema → Task 6). Regression → Task 8.
- **Deferred:** cross-repo `consumed_by` is intentionally a constant marker (`consumedByDeferred`), not implemented — matches the delta spec.
- **Type consistency:** `ProtoFileBundle`/`ProtoServiceBundle`/`ProtoRPC`/`ProtoField` names are shared store types used identically across Tasks 1, 5, 6, 7. `RPCImplLink` (store) is produced by `link.MatchRPCs` and consumed by `store.LinkRPCImpls`. `SchemaInfo` is the new `FindSchema` return type; Task 6 flags the `openapi_tools_test.go` update this requires.
- **Known follow-up during execution:** the exact Graphify `source_location` format is unverified; `signatureMatches` reads the whole source file and scans lines, so it does not depend on `source_location` precision. If real Graphify graphs label gRPC methods differently than the bare RPC name (e.g. `Server.ReserveProduct`), revisit `NodesByLabel` matching in Task 4 during verification on a real fixture.
