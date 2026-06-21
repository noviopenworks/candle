---
change: add-openapi-handler-linking
design-doc: docs/superpowers/specs/2026-06-21-openapi-handler-linking-design.md
base-ref: 7181f9d43a83f5223ce69063009c3b2a2b60162b
---

# OpenAPI Handler Linking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Make `explain_endpoint` answer "which handler implements this endpoint?" by adding an OpenAPI-operation → handler-symbol linker that mirrors the proven gRPC `MatchRPCs` pipeline end to end.

**Architecture:** Ingest runs a new `link.MatchOpenAPI` alongside `link.MatchRPCs`; it discovers handler nodes by operationId-derived name candidates, scores each on a three-tier ladder (LOW name / MEDIUM name+route-presence / HIGH name+AST-confirmed-handler-shape, with a string-scan fallback for HIGH when no `root`), and persists links into a new `http_operation_impls` table keyed by `(index_id, method, path)`. `ExplainEndpoint` reads those links back and returns them as `implemented_by[]`.

**Tech Stack:** Go 1.26, SQLite (mattn/go-sqlite3 via `internal/store`), `go/ast`+`go/parser` for handler-shape confirmation, the MCP Go SDK (`github.com/modelcontextprotocol/go-sdk/mcp`).

## Global Constraints

- Module path is `github.com/noviopenworks/candlegraph`; all imports use it verbatim.
- Confidence tiers are the existing `link` package constants: `confHigh = 0.9`, `confMedium = 0.6`, `confLow = 0.3` (`internal/link/link.go:25-29`). Do NOT introduce new tier values.
- The HTTP linker MUST mirror `MatchRPCs` behavior exactly: every name candidate becomes a link; ambiguous matches keep their tier rather than being dropped or collapsed (`internal/link/link.go:31-55`).
- AST is authoritative: a name match whose declaration is NOT an HTTP handler (e.g. a same-named domain-service method) MUST stay at LOW/MEDIUM and MUST NOT be promoted to HIGH.
- An operation with no `operationId` contributes no link; no node matching the derived name contributes no link. Neither is an error.
- `LinkHTTPOpImpls` replaces an index's HTTP links idempotently (re-indexing a repo is idempotent project-wide), mirroring `LinkRPCImpls` (`internal/store/proto.go:167-191`).
- `ExplainEndpoint`'s existing contract fields are unchanged; `implemented_by` is an additive field that is an empty list (never null-only/error) when no link exists.
- Keep the baseline green after every task: `go build ./...`, `go vet ./...`, `go test ./...`, and `mise exec -- task ci`.
- Tests are table/round-trip style mirroring the existing files; do not add new test frameworks. Commit after every task — never batch.

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `internal/store/schema.go` | Embedded DDL — add `http_operation_impls` table + index | Modify |
| `internal/store/api.go` | `HTTPOpImplLink` type, `LinkHTTPOpImpls` writer, `HTTPOpImpls` reader | Modify |
| `internal/store/api_test.go` | Round-trip test for the writer/reader | Modify |
| `internal/link/openapi.go` | `Op` type, `MatchOpenAPI`, `handlerNameCandidates`, `classifyHTTPHandler`, `hasRouteRegistration`, AST + string-scan helpers, `scoreHTTP` | Create |
| `internal/link/openapi_test.go` | Unit tests for `MatchOpenAPI` tiers | Create |
| `internal/link/testdata/repo/internal/http/handler.go` | Real Go handler fixture for AST confirmation | Create |
| `internal/ingest/ingest.go` | Build `[]link.Op`, call `MatchOpenAPI`, persist via `LinkHTTPOpImpls` | Modify |
| `internal/mcp/openapi_tools.go` | `HTTPOpImpl` type, `EndpointExplanation` result, `ExplainEndpoint` returns `implemented_by` | Modify |
| `internal/mcp/context_tools.go` | Replace stale "not yet available" limitation string | Modify |
| `internal/mcp/e2e_surface_test.go` | Assert HTTP `implemented_by` HIGH for `reserveProduct` | Modify |
| `internal/mcp/e2e_test.go` | Keep `explain_endpoint` assertion compatible with new result shape | Modify |
| `Roadmap.md` | Flip item 0.2 status 🔎 → ✅ | Modify |
| `docs/design.md`, `docs/concepts.md`, `docs/getting-started.md` | Reconcile "deferred"/count drift introduced by the new field | Modify |

---

## Task 1: Store table + migration for HTTP operation impl links

Adds the `http_operation_impls` table to the embedded schema, keyed by the operation identity `(index_id, method, path)` that `OperationByMethodPath` already uses (`internal/store/api.go:181-193`). Mirrors the `proto_rpc_impls` table (`internal/store/schema.go:131-143`).

**Files:**
- Modify: `internal/store/schema.go:130-143` (add table + index after `proto_rpc_impls`)
- Test: deferred to Task 2 (the table is exercised by the writer/reader round-trip)

**Interfaces:**
- Produces: a `http_operation_impls` table with columns `id, index_id, method, path, node_id, confidence, match_reason` and index `idx_http_op_impls_lookup ON http_operation_impls(index_id, method, path)`.

- [x] **Step 1: Add the table to the embedded DDL**

In `internal/store/schema.go`, immediately after the `proto_rpc_impls` index line `CREATE INDEX IF NOT EXISTS idx_proto_rpc_impls_rpc ON proto_rpc_impls(proto_rpc_id);` (line 143), insert:

```sql
CREATE TABLE IF NOT EXISTS http_operation_impls (
  id           INTEGER PRIMARY KEY,
  index_id     INTEGER NOT NULL REFERENCES indexes(id),
  method       TEXT NOT NULL,
  path         TEXT NOT NULL,
  node_id      TEXT NOT NULL,
  confidence   REAL NOT NULL,
  match_reason TEXT
);
CREATE INDEX IF NOT EXISTS idx_http_op_impls_lookup ON http_operation_impls(index_id, method, path);
```

(Insert these lines inside the backtick string literal; keep the trailing backtick on its own line at `schema.go:185`.)

- [x] **Step 2: Verify the schema still compiles and applies**

Run: `go test ./internal/store/ -run TestStore -v`
Expected: PASS (the existing `store_test.go` opens an in-memory DB and applies `schemaSQL`; a malformed DDL would fail `Open`).

- [x] **Step 3: Commit**

```bash
git add internal/store/schema.go
git commit -m "feat(store): add http_operation_impls table keyed by (index_id, method, path)"
```

---

## Task 2: Store writer + reader for HTTP impl links

Adds `HTTPOpImplLink`, `LinkHTTPOpImpls` (idempotent replace), and `HTTPOpImpls` (read by method+path), mirroring `RPCImplLink`/`LinkRPCImpls`/`ProtoRPCImpls` (`internal/store/proto.go:98-104`, `:167-191`, `:294-314`). Unlike proto (which joins through `proto_rpcs`), HTTP links store `index_id`/`method`/`path` directly, so no JOIN is needed.

**Files:**
- Modify: `internal/store/api.go` (append the type + two methods at end of file, after `FindSchemas` at line 229)
- Test: `internal/store/api_test.go` (append a round-trip test mirroring `TestLinkRPCImplsRoundTrip` at `internal/store/proto_test.go:60-83`)

**Interfaces:**
- Consumes: `http_operation_impls` table from Task 1.
- Produces:
  - `type HTTPOpImplLink struct { Method string; Path string; NodeID string; Confidence float64; MatchReason string }`
  - `func (s *Store) LinkHTTPOpImpls(indexID int64, links []HTTPOpImplLink) error`
  - `func (s *Store) HTTPOpImpls(indexID int64, method, path string) ([]HTTPOpImplLink, error)` — matches case-insensitively on method (`UPPER(method)=UPPER(?)`) and exactly on path, mirroring `OperationByMethodPath` (`internal/store/api.go:181-193`).

- [x] **Step 1: Write the failing test**

Append to `internal/store/api_test.go`:

```go
func TestLinkHTTPOpImplsRoundTrip(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")

	links := []HTTPOpImplLink{{
		Method: "POST", Path: "/products/{id}/reservations",
		NodeID: "h1", Confidence: 0.9, MatchReason: "name+route+ast"}}
	if err := s.LinkHTTPOpImpls(id, links); err != nil {
		t.Fatalf("link: %v", err)
	}

	// Method match is case-insensitive; path is exact.
	got, err := s.HTTPOpImpls(id, "post", "/products/{id}/reservations")
	if err != nil || len(got) != 1 || got[0].NodeID != "h1" || got[0].Confidence < 0.85 {
		t.Fatalf("impls: %+v err=%v", got, err)
	}
	if got[0].MatchReason != "name+route+ast" {
		t.Fatalf("reason: %q", got[0].MatchReason)
	}

	// Re-linking replaces (idempotent, still 1).
	if err := s.LinkHTTPOpImpls(id, links); err != nil {
		t.Fatalf("relink: %v", err)
	}
	got, _ = s.HTTPOpImpls(id, "POST", "/products/{id}/reservations")
	if len(got) != 1 {
		t.Fatalf("expected 1 after relink, got %d: %+v", len(got), got)
	}

	// Unknown path yields no links and no error.
	none, err := s.HTTPOpImpls(id, "POST", "/missing")
	if err != nil || len(none) != 0 {
		t.Fatalf("unknown path: %+v err=%v", none, err)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestLinkHTTPOpImplsRoundTrip -v`
Expected: FAIL — compile error, `s.LinkHTTPOpImpls`/`s.HTTPOpImpls`/`HTTPOpImplLink` undefined.

- [x] **Step 3: Write the implementation**

Append to `internal/store/api.go` (after `FindSchemas`, line 229):

```go
// HTTPOpImplLink is an HTTP operation → handler impl link keyed by (index_id,
// method, path), written by the OpenAPI handler linker.
type HTTPOpImplLink struct {
	Method      string
	Path        string
	NodeID      string
	Confidence  float64
	MatchReason string
}

// LinkHTTPOpImpls replaces all HTTP operation impl links for indexID. Idempotent.
func (s *Store) LinkHTTPOpImpls(indexID int64, links []HTTPOpImplLink) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM http_operation_impls WHERE index_id=?`, indexID); err != nil {
		return err
	}
	for _, l := range links {
		if _, err := tx.Exec(
			`INSERT INTO http_operation_impls(index_id, method, path, node_id, confidence, match_reason)
			 VALUES(?,?,?,?,?,?)`,
			indexID, l.Method, l.Path, l.NodeID, l.Confidence, l.MatchReason); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// HTTPOpImpls returns impl links for an operation (method+path) in indexID.
// Method matches case-insensitively; path matches exactly — the same identity
// OperationByMethodPath uses.
func (s *Store) HTTPOpImpls(indexID int64, method, path string) ([]HTTPOpImplLink, error) {
	rows, err := s.DB.Query(
		`SELECT method, path, node_id, confidence, COALESCE(match_reason,'')
		 FROM http_operation_impls
		 WHERE index_id=? AND UPPER(method)=UPPER(?) AND path=?`, indexID, method, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HTTPOpImplLink
	for rows.Next() {
		var l HTTPOpImplLink
		if err := rows.Scan(&l.Method, &l.Path, &l.NodeID, &l.Confidence, &l.MatchReason); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestLinkHTTPOpImplsRoundTrip -v`
Expected: PASS

- [x] **Step 5: Run the store package + vet**

Run: `go vet ./internal/store/ && go test ./internal/store/`
Expected: PASS

- [x] **Step 6: Commit**

```bash
git add internal/store/api.go internal/store/api_test.go
git commit -m "feat(store): add LinkHTTPOpImpls writer and HTTPOpImpls reader"
```

---

## Task 3: Linker `MatchOpenAPI` with the three-tier scoring ladder

New entry point parallel to `MatchRPCs` (`internal/link/link.go:36-55`). Discovers handler nodes from operationId-derived candidates, computes `hasRouteRegistration` once per call (analog of `hasServiceRegistration`, `internal/link/link.go:57-68`), and scores each candidate with `scoreHTTP` — the three-tier ladder mirroring `score()` (`internal/link/link.go:78-102`). AST confirmation reuses `readSourceUnderRoot` (`internal/link/link.go:352-375`) and the `fieldTypes`/`typeName`/`isContextContext` helpers; the no-root path falls back to a string-scan sibling of `signatureMatches` (`internal/link/link.go:239-275`).

The HTTP handler shape is `func (recv) Name(w http.ResponseWriter, r *http.Request)` — exactly two params flattening to `[http.ResponseWriter, *http.Request]`, no results. Because every handler shares this signature, the AST gate confirms "this is A handler" — the **name** carries "this is THE handler." A same-named non-handler (e.g. `func (s *Service) ReserveProduct(ctx context.Context, ...) (...)`) fails `classifyHTTPHandler` and stays at LOW/MEDIUM.

**Files:**
- Create: `internal/link/openapi.go`
- Create: `internal/link/testdata/repo/internal/http/handler.go` (real handler fixture; the testdata repo root is `internal/link/testdata/repo`, per `internal/link/ast_test.go:11`)

**Interfaces:**
- Consumes: `store.Store.NodesByLabel` (`internal/store/query.go:41-47`), `store.NodeRow` (`internal/store/query.go:3-11`), `store.HTTPOpImplLink` (Task 2), and the existing `link` constants/helpers (`confHigh/confMedium/confLow`, `readSourceUnderRoot`, `fieldTypes`, `typeName`, `isContextContext`).
- Produces:
  - `type Op struct { Method string; Path string; OperationID string }`
  - `func MatchOpenAPI(s *store.Store, indexID int64, ops []Op, root string) ([]store.HTTPOpImplLink, error)`

- [x] **Step 1: Write the handler testdata fixture**

Create `internal/link/testdata/repo/internal/http/handler.go`:

```go
package http

import "net/http"

// Handler holds HTTP handlers.
type Handler struct{}

// ReserveProduct is a real HTTP handler (the AST gate must confirm this shape).
func (h *Handler) ReserveProduct(w http.ResponseWriter, r *http.Request) {}

// Service is a same-named domain method that is NOT an HTTP handler.
type Service struct{}

// ReserveProductDomain has a different name to avoid colliding in the same file;
// the same-named non-handler case is exercised with an inline fixture in the test.
func (s *Service) ReserveProductDomain(req string) (string, error) { return "", nil }
```

- [x] **Step 2: Write the failing test (full test body is in Task 4)**

Task 4 holds the test file; this step only confirms the fixture compiles standalone.

Run: `go build ./internal/link/testdata/...` (will error harmlessly if the dir is not a buildable package — that's fine; testdata is excluded from normal builds). Instead verify with: `gofmt -l internal/link/testdata/repo/internal/http/handler.go`
Expected: no output (file is gofmt-clean).

- [x] **Step 3: Write `internal/link/openapi.go`**

```go
package link

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/noviopenworks/candlegraph/internal/store"
)

// Op is the subset of an HTTP operation the linker needs.
type Op struct {
	Method      string
	Path        string
	OperationID string
}

// MatchOpenAPI returns handler impl links for ops within a single index. Each
// name candidate becomes a link; ambiguous matches keep their tier rather than
// being dropped or collapsed, mirroring MatchRPCs. root is the absolute source
// root used to resolve node source files for AST analysis; an empty root
// disables AST and falls back to the string-scan heuristic.
func MatchOpenAPI(s *store.Store, indexID int64, ops []Op, root string) ([]store.HTTPOpImplLink, error) {
	routeRegistered, err := hasRouteRegistration(s, indexID)
	if err != nil {
		return nil, err
	}
	var out []store.HTTPOpImplLink
	for _, op := range ops {
		for _, name := range handlerNameCandidates(op.OperationID) {
			nodes, err := s.NodesByLabel(indexID, name)
			if err != nil {
				return nil, err
			}
			for _, n := range nodes {
				conf, reason := scoreHTTP(root, n, name, routeRegistered)
				out = append(out, store.HTTPOpImplLink{
					Method: op.Method, Path: op.Path, NodeID: n.NodeID,
					Confidence: conf, MatchReason: reason,
				})
			}
		}
	}
	return out, nil
}

// handlerNameCandidates derives handler-name candidates from operationId only:
// the operationId verbatim and its PascalCase (exported) form, deduped. An empty
// operationId yields no candidates → the op contributes no link.
func handlerNameCandidates(operationID string) []string {
	if operationID == "" {
		return nil
	}
	out := []string{operationID}
	if title := titleFirst(operationID); title != operationID {
		out = append(out, title)
	}
	return out
}

// titleFirst upper-cases the first rune of s (ASCII), e.g. "reserveProduct" ->
// "ReserveProduct". It does not touch the remainder, matching Go exported-method
// naming where only the leading rune differs from a camelCase operationId.
func titleFirst(s string) string {
	if s == "" {
		return s
	}
	c := s[0]
	if c >= 'a' && c <= 'z' {
		return string(c-('a'-'A')) + s[1:]
	}
	return s
}

// hasRouteRegistration is a coarse, existence-based signal analogous to
// hasServiceRegistration: it reports whether the repo contains any HTTP
// route-registration infrastructure node. It is computed once per MatchOpenAPI
// call, not per op. Precise path→handler binding is deferred.
func hasRouteRegistration(s *store.Store, indexID int64) (bool, error) {
	for _, label := range []string{"HandleFunc", "Handle", "NewServeMux", "NewRouter", "registerRoutes"} {
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

// scoreHTTP maps the available signals to a confidence tier and reason string,
// mirroring score() for RPCs:
//   - name alone                      → LOW  "name"
//   - + route-registration presence   → MEDIUM "name+route"
//   - + AST-confirmed handler shape    → HIGH "...+ast" (root available)
//   - + string-scan confirms shape     → HIGH "...+signature" (root absent)
// AST is authoritative: when the source is readable but the declaration is not
// a handler, the candidate keeps its name/route tier and is never promoted.
func scoreHTTP(root string, n store.NodeRow, name string, routeRegistered bool) (float64, string) {
	reason := "name"
	conf := confLow
	if routeRegistered {
		reason = "name+route"
		conf = confMedium
	}

	matched, ok := astHTTPHandlerMatch(root, n.SourceFile, name)
	if ok {
		if matched {
			reason += "+ast"
			conf = confHigh
		}
		return conf, reason
	}

	if httpSignatureScan(n.SourceFile, name) {
		reason += "+signature"
		conf = confHigh
	}
	return conf, reason
}

// astHTTPHandlerMatch parses the node's source under root and reports whether a
// method named name is an HTTP handler. ok=false means the source was
// unavailable (caller falls back to the string scan); ok=true with matched=false
// means the source parsed but no such handler declaration exists.
func astHTTPHandlerMatch(root, sourceFile, name string) (matched bool, ok bool) {
	path, src, ok := readSourceUnderRoot(root, sourceFile)
	if !ok {
		return false, false
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return false, false
	}
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || fn.Name == nil || fn.Name.Name != name {
			continue
		}
		if classifyHTTPHandler(fn) {
			return true, true
		}
	}
	return false, true
}

// classifyHTTPHandler reports whether fn has the HTTP handler shape: exactly two
// params flattening to [http.ResponseWriter, *http.Request] and no results.
func classifyHTTPHandler(fn *ast.FuncDecl) bool {
	params := fieldTypes(fn.Type.Params)
	if len(params) != 2 {
		return false
	}
	if fn.Type.Results != nil && len(fn.Type.Results.List) != 0 {
		return false
	}
	if !isSelector(params[0], "http", "ResponseWriter") {
		return false
	}
	star, ok := params[1].(*ast.StarExpr)
	if !ok {
		return false
	}
	return isSelector(star.X, "http", "Request")
}

// isSelector reports whether expr is the selector pkg.sel (e.g. http.Request).
func isSelector(expr ast.Expr, pkg, sel string) bool {
	s, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	return ok && id.Name == pkg && s.Sel.Name == sel
}

// httpSignatureScan is the legacy fallback used only when AST is unavailable
// (no root). It reads the node's source_file directly and looks for a func
// declaration of name whose params mention http.ResponseWriter and *http.Request.
// Unreadable or unsafe paths simply do not match. Sibling of signatureMatches.
func httpSignatureScan(sourceFile, name string) bool {
	if sourceFile == "" {
		return false
	}
	// #nosec G304 -- legacy fallback intentionally reads graph source_file when no
	// repo root was configured; unreadable paths simply do not match.
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "func") || !strings.Contains(line, name+"(") {
			continue
		}
		params := line
		if i := strings.Index(line, name+"("); i >= 0 {
			params = line[i+len(name):]
		}
		if strings.Contains(params, "http.ResponseWriter") &&
			strings.Contains(params, "*http.Request") {
			return true
		}
	}
	return false
}
```

- [x] **Step 4: Run package build + vet**

Run: `go build ./internal/link/ && go vet ./internal/link/`
Expected: PASS (no test yet — Task 4 adds tests).

- [x] **Step 5: Commit**

```bash
git add internal/link/openapi.go internal/link/testdata/repo/internal/http/handler.go
git commit -m "feat(link): add MatchOpenAPI handler linker with three-tier scoring"
```

---

## Task 4: Unit tests for `MatchOpenAPI` (all tiers)

Covers every delta-spec scenario (`openspec/changes/add-openapi-handler-linking/specs/ast-linking/spec.md`): HIGH via AST, HIGH via string-scan (no root), MEDIUM via route presence, LOW for a same-named non-handler, no operationId → no link, no candidate → no link. Mirrors `TestMatchRPCsConfidence`/`TestMatchRPCsAmbiguousLowConfidence` (`internal/link/link_test.go`), reusing the `mustNode` helper already in that package (`internal/link/link_test.go:115-121`).

**Files:**
- Create: `internal/link/openapi_test.go`

**Interfaces:**
- Consumes: `MatchOpenAPI`, `Op` (Task 3), `store.HTTPOpImplLink` (Task 2), the package-local `mustNode` helper, and the AST fixture `internal/link/testdata/repo/internal/http/handler.go` (Task 3, root = `internal/link/testdata/repo`).

- [x] **Step 1: Write the failing tests**

Create `internal/link/openapi_test.go`:

```go
package link

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/noviopenworks/candlegraph/internal/store"
)

// TestMatchOpenAPIHighViaAST: a handler whose go/ast declaration is a real HTTP
// handler under root earns HIGH with an "ast" reason.
func TestMatchOpenAPIHighViaAST(t *testing.T) {
	root := filepath.Join("testdata", "repo")
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "h1", "ReserveProduct", "internal/http/handler.go")

	ops := []Op{{Method: "POST", Path: "/products/{id}/reservations", OperationID: "reserveProduct"}}
	links, err := MatchOpenAPI(s, id, ops, root)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	// operationId "reserveProduct" → candidates {reserveProduct, ReserveProduct};
	// the PascalCase candidate matches node h1.
	var hit *store.HTTPOpImplLink
	for i := range links {
		if links[i].NodeID == "h1" {
			hit = &links[i]
		}
	}
	if hit == nil || hit.Confidence < 0.85 {
		t.Fatalf("expected HIGH link to h1, got: %+v", links)
	}
	if hit.Method != "POST" || hit.Path != "/products/{id}/reservations" {
		t.Fatalf("identity not carried: %+v", hit)
	}
}

// TestMatchOpenAPIHighViaStringScan: no root, but the node's source_file is
// directly readable and matches the handler shape → HIGH via "+signature".
func TestMatchOpenAPIHighViaStringScan(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "handler.go")
	code := "package h\n" +
		"import \"net/http\"\n" +
		"func (h *Handler) ReserveProduct(w http.ResponseWriter, r *http.Request) {}\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "h1", "ReserveProduct", src)

	ops := []Op{{Method: "POST", Path: "/x", OperationID: "ReserveProduct"}}
	links, err := MatchOpenAPI(s, id, ops, "") // root="" disables AST
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if len(links) != 1 || links[0].NodeID != "h1" || links[0].Confidence < 0.85 {
		t.Fatalf("expected HIGH via string-scan, got: %+v", links)
	}
}

// TestMatchOpenAPIMediumViaRoute: route-registration presence but no AST
// confirmation (no root, unreadable source) → MEDIUM "name+route".
func TestMatchOpenAPIMediumViaRoute(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "h1", "ReserveProduct", "/nonexistent/handler.go")
	mustNode(t, s, id, "r1", "HandleFunc", "/nonexistent/router.go") // route presence

	ops := []Op{{Method: "POST", Path: "/x", OperationID: "ReserveProduct"}}
	links, err := MatchOpenAPI(s, id, ops, "")
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if len(links) != 1 || links[0].Confidence != 0.6 || links[0].MatchReason != "name+route" {
		t.Fatalf("expected MEDIUM name+route, got: %+v", links)
	}
}

// TestMatchOpenAPILowForNonHandler: a same-named domain-service method (not an
// HTTP handler) with no route presence stays LOW and is never promoted to HIGH.
func TestMatchOpenAPILowForNonHandler(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "service.go")
	code := "package svc\n" +
		"import \"context\"\n" +
		"func (s *Service) ReserveProduct(ctx context.Context, req *Request) (*Reservation, error) { return nil, nil }\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	root := dir
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "n1", "ReserveProduct", "service.go") // resolves under root=dir

	ops := []Op{{Method: "POST", Path: "/x", OperationID: "ReserveProduct"}}
	links, err := MatchOpenAPI(s, id, ops, root)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if len(links) != 1 || links[0].Confidence != 0.3 || links[0].MatchReason != "name" {
		t.Fatalf("expected LOW name (no AST promotion), got: %+v", links)
	}
}

// TestMatchOpenAPINoLink: no operationId → no link; a candidate with no matching
// node → no link. Neither errors.
func TestMatchOpenAPINoLink(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "h1", "ReserveProduct", "/x/handler.go")

	// No operationId → no candidates → no link.
	ops := []Op{{Method: "GET", Path: "/health", OperationID: ""}}
	links, err := MatchOpenAPI(s, id, ops, "")
	if err != nil || len(links) != 0 {
		t.Fatalf("no-operationId: %+v err=%v", links, err)
	}

	// operationId with no matching node → no link.
	ops = []Op{{Method: "GET", Path: "/ghost", OperationID: "ghostHandler"}}
	links, err = MatchOpenAPI(s, id, ops, "")
	if err != nil || len(links) != 0 {
		t.Fatalf("no-candidate: %+v err=%v", links, err)
	}
}
```

- [x] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/link/ -run TestMatchOpenAPI -v`
Expected: PASS for all five tests.

(If `TestMatchOpenAPILowForNonHandler` fails because the node's `source_file` does not resolve under `root`, confirm `mustNode` stores `source_file` exactly `"service.go"` and `root` is the temp dir — `readSourceUnderRoot` joins them. The fixture name must match.)

- [x] **Step 3: Run the full link package + vet**

Run: `go vet ./internal/link/ && go test ./internal/link/`
Expected: PASS (existing RPC/export tests still green).

- [x] **Step 4: Commit**

```bash
git add internal/link/openapi_test.go
git commit -m "test(link): cover MatchOpenAPI tiers (AST HIGH, scan HIGH, MEDIUM, LOW, no-link)"
```

---

## Task 5: Ingest wiring — run `MatchOpenAPI` alongside `MatchRPCs`

Builds `[]link.Op` from the indexed OpenAPI bundles, calls `MatchOpenAPI` gated on `r.Root` (same as RPC linking), and persists via `LinkHTTPOpImpls`. Insert right after `ReplaceAPISpecs` succeeds (`internal/ingest/ingest.go:68-70`), before the protobuf block — mirroring how `MatchRPCs`/`LinkRPCImpls` sit after `ReplaceProtoFiles` (`internal/ingest/ingest.go:80-93`).

**Files:**
- Modify: `internal/ingest/ingest.go` (insert linking after line 70; add a `collectOps` helper near `collectRPCs` at line 161)
- Test: `internal/ingest/ingest_test.go` (extend the existing ingest test; see Step 1)

**Interfaces:**
- Consumes: `link.MatchOpenAPI`, `link.Op` (Task 3), `store.LinkHTTPOpImpls` (Task 2), the `bundles []store.APISpecBundle` already built at `internal/ingest/ingest.go:59-67`, and `r.Root`.
- Produces: persisted `http_operation_impls` rows after each repo is indexed.

- [x] **Step 1: Write the failing test**

Inspect `internal/ingest/ingest_test.go` first (it already asserts RPC `implemented_by` per the codegraph blast-radius note). Add a focused test that indexes a fixture with an OpenAPI op + a handler node + handler source under root, then asserts an HTTP impl link is persisted. Append:

```go
func TestIngestLinksHTTPHandler(t *testing.T) {
	dir := t.TempDir()
	// Handler source under the repo root so AST confirms the handler shape.
	if err := os.MkdirAll(filepath.Join(dir, "internal", "http"), 0o755); err != nil {
		t.Fatal(err)
	}
	handler := "package http\nimport \"net/http\"\n" +
		"type Handler struct{}\n" +
		"func (h *Handler) ReserveProduct(w http.ResponseWriter, r *http.Request) {}\n"
	if err := os.WriteFile(filepath.Join(dir, "internal", "http", "handler.go"), []byte(handler), 0o644); err != nil {
		t.Fatal(err)
	}
	graphJSON := `{"nodes":[{"id":"h1","label":"ReserveProduct","file_type":"code","source_file":"internal/http/handler.go"}],"edges":[],"hyperedges":[]}`
	graphPath := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(graphPath, []byte(graphJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := "openapi: 3.0.3\ninfo:\n  title: I\n  version: \"1\"\n" +
		"paths:\n  /x:\n    post:\n      operationId: reserveProduct\n" +
		"      responses:\n        '200': { description: ok }\n"
	specPath := filepath.Join(dir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}

	s, _ := store.Open(":memory:")
	defer s.Close()
	cfg := &config.Config{Repos: []config.Repo{{
		Repo: "org/svc", Graph: graphPath, Commit: "abc", Branch: "main", Root: dir,
		OpenAPI: []string{specPath},
	}}}
	if _, err := Run(s, cfg); err != nil {
		t.Fatalf("run: %v", err)
	}

	ri, _ := s.UpsertIndex("org", "svc", "abc", "main", graphPath)
	impls, err := s.HTTPOpImpls(ri, "POST", "/x")
	if err != nil || len(impls) == 0 {
		t.Fatalf("expected HTTP impl link, got %+v err=%v", impls, err)
	}
	if impls[0].NodeID != "h1" || impls[0].Confidence < 0.85 {
		t.Fatalf("expected HIGH link to h1, got %+v", impls)
	}
}
```

Add `"os"`, `"path/filepath"`, and the `config`/`store` imports to the test file's import block if not already present (match the existing imports in `internal/ingest/ingest_test.go`; the `config.Repo` field names — `Repo`, `Graph`, `Commit`, `Branch`, `Root`, `OpenAPI` — must match `internal/config`; verify exact field names there before writing).

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ingest/ -run TestIngestLinksHTTPHandler -v`
Expected: FAIL — `s.HTTPOpImpls` returns 0 links (ingest does not link HTTP yet).

- [x] **Step 3: Add the `collectOps` helper**

In `internal/ingest/ingest.go`, after `collectRPCs` (ends line 172), add:

```go
func collectOps(bundles []store.APISpecBundle) []link.Op {
	var out []link.Op
	for _, b := range bundles {
		for _, op := range b.Operations {
			out = append(out, link.Op{Method: op.Method, Path: op.Path, OperationID: op.OperationID})
		}
	}
	return out
}
```

- [x] **Step 4: Wire the linker into `Run`**

In `internal/ingest/ingest.go`, replace the `ReplaceAPISpecs` block (lines 68-70):

```go
		if err := s.ReplaceAPISpecs(indexID, bundles); err != nil {
			return rep, err
		}
```

with:

```go
		if err := s.ReplaceAPISpecs(indexID, bundles); err != nil {
			return rep, err
		}
		ops := collectOps(bundles)
		if len(ops) > 0 && r.Root == "" {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: no source root configured; HTTP handler links use heuristic (AST precision off)", r.Repo))
		}
		opLinks, err := link.MatchOpenAPI(s, indexID, ops, r.Root)
		if err != nil {
			return rep, err
		}
		if err := s.LinkHTTPOpImpls(indexID, opLinks); err != nil {
			return rep, err
		}
```

- [x] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/ingest/ -run TestIngestLinksHTTPHandler -v`
Expected: PASS

- [x] **Step 6: Run the ingest package + vet**

Run: `go vet ./internal/ingest/ && go test ./internal/ingest/`
Expected: PASS (existing ingest tests still green).

- [x] **Step 7: Commit**

```bash
git add internal/ingest/ingest.go internal/ingest/ingest_test.go
git commit -m "feat(ingest): run MatchOpenAPI and persist HTTP handler links"
```

---

## Task 6: MCP `ExplainEndpoint` returns `implemented_by[]`

`ExplainEndpoint` currently returns the bare `store.HTTPOperation` (`internal/mcp/openapi_tools.go:52-69`). Wrap it in an `EndpointExplanation` result that adds `implemented_by []HTTPOpImpl`, mirroring `ExplainRPC`/`RPCExplanation` (`internal/mcp/proto_tools.go:16-71`). The confidence float is rendered as a tier string (HIGH/MEDIUM/LOW) for the agent-facing surface, matching the design's `HTTPOpImpl` shape.

**Files:**
- Modify: `internal/mcp/openapi_tools.go` (add `HTTPOpImpl`, `EndpointExplanation`, a `tierLabel` helper, change `ExplainEndpoint`'s signature + body)
- Test: `internal/mcp/openapi_tools_test.go` (extend; the file already covers `ExplainEndpoint` per the codegraph blast-radius note)

**Interfaces:**
- Consumes: `store.HTTPOpImpls` (Task 2), `store.HTTPOperation` (`internal/store/api.go:23-34`).
- Produces:
  - `type HTTPOpImpl struct { Symbol string \`json:"symbol"\`; Confidence string \`json:"confidence"\`; Reason string \`json:"reason"\` }`
  - `type EndpointExplanation struct { Operation store.HTTPOperation \`json:"operation"\`; ImplementedBy []HTTPOpImpl \`json:"implemented_by"\` }`
  - `func (t *Tools) ExplainEndpoint(repo, method, path string) (EndpointExplanation, error)` — `ImplementedBy` is an empty (non-nil) slice when no link.

> NOTE: `ExplainEndpoint`'s return type changes from `store.HTTPOperation` to `EndpointExplanation`. Update every caller — search with `grep -rn "ExplainEndpoint" internal/ cmd/` and adjust (the MCP server registration in the `serve` wiring and any tests). The operation moves under `.Operation`.

- [x] **Step 1: Write the failing test**

In `internal/mcp/openapi_tools_test.go`, add (and adapt the existing `ExplainEndpoint` test to the new `.Operation` field):

```go
func TestExplainEndpointImplementedBy(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	if err := s.ReplaceAPISpecs(id, []store.APISpecBundle{{
		Spec:       store.APISpec{Kind: "openapi", Name: "I", Version: "1", Path: "api/openapi.yaml"},
		Operations: []store.HTTPOperation{{Method: "POST", Path: "/x", OperationID: "reserveProduct"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkHTTPOpImpls(id, []store.HTTPOpImplLink{{
		Method: "POST", Path: "/x", NodeID: "h1", Confidence: 0.9, MatchReason: "name+route+ast"}}); err != nil {
		t.Fatal(err)
	}
	tools := NewTools(s)

	expl, err := tools.ExplainEndpoint("acme/inventory", "POST", "/x")
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if expl.Operation.OperationID != "reserveProduct" {
		t.Fatalf("operation not returned: %+v", expl.Operation)
	}
	if len(expl.ImplementedBy) != 1 || expl.ImplementedBy[0].Symbol != "h1" || expl.ImplementedBy[0].Confidence != "HIGH" {
		t.Fatalf("implemented_by: %+v", expl.ImplementedBy)
	}

	// No link → empty (non-nil) slice, contract still returned.
	if err := s.ReplaceAPISpecs(id, []store.APISpecBundle{{
		Spec:       store.APISpec{Kind: "openapi", Name: "I", Version: "1", Path: "api/openapi.yaml"},
		Operations: []store.HTTPOperation{{Method: "GET", Path: "/y", OperationID: "noimpl"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkHTTPOpImpls(id, nil); err != nil {
		t.Fatal(err)
	}
	expl, err = tools.ExplainEndpoint("acme/inventory", "GET", "/y")
	if err != nil {
		t.Fatalf("explain noimpl: %v", err)
	}
	if expl.ImplementedBy == nil || len(expl.ImplementedBy) != 0 {
		t.Fatalf("expected empty non-nil implemented_by, got %#v", expl.ImplementedBy)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run TestExplainEndpointImplementedBy -v`
Expected: FAIL — `expl.Operation`/`expl.ImplementedBy` undefined (return type is still `store.HTTPOperation`).

- [x] **Step 3: Rewrite `ExplainEndpoint`**

In `internal/mcp/openapi_tools.go`, replace the `ExplainEndpoint` method (lines 52-69) with:

```go
// HTTPOpImpl is one handler implementation link for explain_endpoint.
type HTTPOpImpl struct {
	Symbol     string `json:"symbol"`
	Confidence string `json:"confidence"` // HIGH | MEDIUM | LOW
	Reason     string `json:"reason"`
}

// EndpointExplanation is the explain_endpoint result: contract data plus the
// AST-linked handler symbol(s).
type EndpointExplanation struct {
	Operation     store.HTTPOperation `json:"operation"`
	ImplementedBy []HTTPOpImpl        `json:"implemented_by"`
}

// ExplainEndpoint implements explain_endpoint: contract data plus same-repo
// handler impl links (empty list when none).
func (t *Tools) ExplainEndpoint(repo, method, path string) (EndpointExplanation, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return EndpointExplanation{}, err
	}
	if !ok {
		return EndpointExplanation{}, ErrNotFound
	}
	op, found, err := t.s.OperationByMethodPath(ri.IndexID, method, path)
	if err != nil {
		return EndpointExplanation{}, err
	}
	if !found {
		return EndpointExplanation{}, ErrNotFound
	}
	out := EndpointExplanation{Operation: op, ImplementedBy: []HTTPOpImpl{}}
	links, err := t.s.HTTPOpImpls(ri.IndexID, method, path)
	if err != nil {
		return EndpointExplanation{}, err
	}
	for _, l := range links {
		out.ImplementedBy = append(out.ImplementedBy, HTTPOpImpl{
			Symbol: l.NodeID, Confidence: tierLabel(l.Confidence), Reason: l.MatchReason})
	}
	return out, nil
}

// tierLabel maps a confidence float to its agent-facing tier name.
func tierLabel(conf float64) string {
	switch {
	case conf >= 0.85:
		return "HIGH"
	case conf >= 0.5:
		return "MEDIUM"
	default:
		return "LOW"
	}
}
```

- [x] **Step 4: Fix every `ExplainEndpoint` caller**

Run: `grep -rn "ExplainEndpoint" internal/ cmd/`
For each caller outside `openapi_tools.go` (the MCP tool registration in the `serve` wiring, and any test), the result is now `EndpointExplanation`; references to the operation become `.Operation`. Update them.

- [x] **Step 5: Run the test + package**

Run: `go test ./internal/mcp/ -run TestExplainEndpointImplementedBy -v && go vet ./internal/mcp/`
Expected: PASS

- [x] **Step 6: Commit**

```bash
git add internal/mcp/openapi_tools.go internal/mcp/openapi_tools_test.go
git commit -m "feat(mcp): explain_endpoint returns implemented_by handler links"
```

---

## Task 7: Update stale capability/limitation strings

`contextLimitations()` still advertises "OpenAPI endpoint implementation linking is not yet available in get_context v1." (`internal/mcp/context_tools.go:259-266`). With HTTP linking shipped, remove/replace that line. The OpenAPI `CapabilitySummary` tool list (`internal/mcp/context_tools.go:218`) already lists `explain_endpoint`, so no change is needed there.

**Files:**
- Modify: `internal/mcp/context_tools.go:259-266`
- Test: `internal/mcp/context_tools_test.go` (extend if it asserts the limitation strings; see Step 1)

**Interfaces:**
- Consumes: nothing new.
- Produces: a `contextLimitations()` slice with no "OpenAPI endpoint implementation linking is not yet available" entry.

- [x] **Step 1: Write/adjust the failing test**

In `internal/mcp/context_tools_test.go`, add an assertion that the stale string is gone:

```go
func TestContextLimitationsNoStaleOpenAPINote(t *testing.T) {
	for _, lim := range contextLimitations() {
		if strings.Contains(lim, "OpenAPI endpoint implementation linking is not yet available") {
			t.Fatalf("stale limitation still present: %q", lim)
		}
	}
}
```

Ensure `"strings"` and `"testing"` are imported in that test file.

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run TestContextLimitationsNoStaleOpenAPINote -v`
Expected: FAIL — the stale string is still in the slice.

- [x] **Step 3: Edit `contextLimitations()`**

In `internal/mcp/context_tools.go`, replace:

```go
func contextLimitations() []string {
	return []string{
		"OpenAPI endpoint implementation linking is not yet available in get_context v1.",
		"Cross-repo RPC consumed_by aggregation is deferred.",
		"Cross-repo private library consumer aggregation is deferred.",
		"Graph traversal is one hop; depth > 1 is deferred.",
	}
}
```

with:

```go
func contextLimitations() []string {
	return []string{
		"OpenAPI/HTTP handler linking is name-based: path→handler binding is coarse (route-registration presence), not router-precise.",
		"Cross-repo RPC consumed_by aggregation is deferred.",
		"Cross-repo private library consumer aggregation is deferred.",
		"Graph traversal is one hop; depth > 1 is deferred.",
	}
}
```

- [x] **Step 4: Run the test + package**

Run: `go test ./internal/mcp/ -run TestContextLimitationsNoStaleOpenAPINote -v && go vet ./internal/mcp/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/mcp/context_tools.go internal/mcp/context_tools_test.go
git commit -m "docs(mcp): replace stale OpenAPI-linking limitation with router-precision caveat"
```

---

## Task 8: End-to-end assertion — HTTP `implemented_by` HIGH for `reserveProduct`

The surface e2e (`internal/mcp/e2e_surface_test.go`) already builds the binary, indexes the `inventory` repo with `root` set, and has a real `internal/http/handler.go`-style fixture pattern. The inventory repo already has an OpenAPI op `reserveProduct` (`e2e_surface_test.go:95-106`). Add an HTTP handler node + real handler source so the linker confirms HIGH, then assert `explain_endpoint` returns `implemented_by` HIGH — mirroring the existing proto assertion at `e2e_surface_test.go:238-240`. Also update the simpler `TestEndToEndStdio` assertion (`e2e_test.go:144-147`) which now reads a wrapped result.

**Files:**
- Modify: `internal/mcp/e2e_surface_test.go` (add handler node + source fixture + assertion)
- Modify: `internal/mcp/e2e_test.go` (assertion still matches the new JSON shape)

**Interfaces:**
- Consumes: the running `serve` subprocess and the `call`/`mustContain` helpers already defined in `e2e_surface_test.go:198-216`.
- Produces: an e2e assertion that `explain_endpoint` for `POST /reservations` returns the handler node at HIGH.

- [x] **Step 1: Add the handler node to the inventory graph fixture**

In `internal/mcp/e2e_surface_test.go`, in the consumer-repo `graph.json` (lines 51-61), add a handler node to the `nodes` array (and an edge so the node is non-isolated, optional):

```json
{"id":"http_reserveproduct","label":"ReserveProduct","file_type":"code","source_file":"internal/http/handler.go","source_location":"L8"}
```

NOTE: the graph already has a node labelled `ReserveProduct` (`grpc_server_reserveproduct`) whose source is `internal/grpc/server.go` (a gRPC unary method, NOT an HTTP handler). That node will also be a name candidate for the operationId `reserveProduct` — it parses under root, fails `classifyHTTPHandler`, and stays LOW/MEDIUM, while the new `http_reserveproduct` node confirms HIGH. The assertion below targets the HIGH node, so both candidates coexisting is expected and correct (mirrors `MatchRPCs` ambiguity behavior).

- [x] **Step 2: Add the real handler source fixture**

After the existing `internal/grpc/server.go` fixture (ends line 76), add:

```go
	writeFixture(t, inv, "internal/http/handler.go", `package http

import "net/http"

type Handler struct{}

func (h *Handler) ReserveProduct(w http.ResponseWriter, r *http.Request) {}
`)
```

- [x] **Step 3: Add the assertion**

After the existing AST-confirmed RPC assertion (`e2e_surface_test.go:238-240`), add:

```go
	// AST-confirmed HTTP handler implementation (HIGH tier for reserveProduct).
	epBody := call("explain_endpoint", map[string]any{"repo": "org/inventory", "method": "POST", "path": "/reservations"})
	mustContain("explain_endpoint", epBody, "http_reserveproduct", "implemented_by", "HIGH")
```

- [x] **Step 4: Keep `TestEndToEndStdio` green**

In `internal/mcp/e2e_test.go`, the existing assertion checks `strings.Contains(epBody, "reserveProduct")` (line 145). The new result nests the operation under `"operation"`, but the operationId string `reserveProduct` is still present in the JSON, so the assertion holds. Verify by running the test (Step 5); no edit expected. If the assertion regresses, change it to look for `"operation"` + `reserveProduct`.

- [x] **Step 5: Run both e2e tests**

Run: `go test ./internal/mcp/ -run 'TestEndToEnd' -v`
Expected: PASS — `TestEndToEndToolSurface` shows the HTTP HIGH link; `TestEndToEndStdio` still passes.

- [x] **Step 6: Commit**

```bash
git add internal/mcp/e2e_surface_test.go internal/mcp/e2e_test.go
git commit -m "test(mcp): e2e asserts explain_endpoint returns HTTP implemented_by HIGH"
```

---

## Task 9: Reconcile docs + flip Roadmap 0.2

Flip Roadmap item 0.2 from 🔎 to ✅ and reconcile the doc drift the new field introduces: the design.md "deferred" framing for OpenAPI handler linking and any concepts/getting-started note that says HTTP linking is unavailable. This is doc-only — no code.

**Files:**
- Modify: `Roadmap.md` (item 0.2 row; the row begins `| 0.2 | **OpenAPI → handler linking.** ...` with status cell `| 🔎 |`)
- Modify: `docs/design.md`, `docs/concepts.md`, `docs/getting-started.md` (reconcile only the notes that claim HTTP handler linking is deferred/unavailable; leave unrelated text)

**Interfaces:**
- Consumes: nothing (docs).
- Produces: Roadmap 0.2 = ✅; no doc claims HTTP `implemented_by` is unavailable.

- [x] **Step 1: Flip Roadmap 0.2 status**

In `Roadmap.md`, in the Phase 0 table row for 0.2, change the status cell from `🔎` to `✅`. The "Why it matters" cell currently says the flagship question *cannot be answered today*; update its tense to reflect that it now can (e.g. "Answers the README's flagship question — *which handler implements this endpoint?* — via name + AST-confirmed handler shape; path→handler binding remains coarse.").

- [x] **Step 2: Reconcile design.md**

Run: `grep -n "implemented_by\|handler linking\|deferred\|MatchOpenAPI" docs/design.md`
For any line that frames OpenAPI handler linking as deferred/future, update it to "implemented (name + AST handler-shape; route binding coarse)." Do NOT touch the unrelated `implemented_by` example payloads (`docs/design.md:116,167,406,509`) — those already show the field and are now accurate.

- [x] **Step 3: Reconcile concepts.md + getting-started.md**

Run: `grep -n "implemented_by\|handler\|explain_endpoint\|deferred" docs/concepts.md docs/getting-started.md`
Update any sentence claiming `explain_endpoint` returns only the contract / that HTTP handler linking is unavailable. (`docs/concepts.md:63` already documents `implemented_by` generically — leave it.) If `getting-started.md` enumerates what `explain_endpoint` returns, add `implemented_by`.

- [x] **Step 4: Sanity check no stale claim remains**

Run: `grep -rn "endpoint implementation linking is not yet\|handler linking.*deferred\|implemented_by.*not.*available" docs/ Roadmap.md`
Expected: no output.

- [x] **Step 5: Commit**

```bash
git add Roadmap.md docs/design.md docs/concepts.md docs/getting-started.md
git commit -m "docs: flip roadmap 0.2 to done; reconcile HTTP handler-linking notes"
```

---

## Task 10: Full baseline verification

Run the complete project verification baseline to confirm the whole change is green before handoff.

**Files:** none (verification only).

- [x] **Step 1: Build**

Run: `go build ./...`
Expected: no output, exit 0.

- [x] **Step 2: Vet**

Run: `go vet ./...`
Expected: no output, exit 0.

- [x] **Step 3: Full test suite**

Run: `go test ./...`
Expected: all packages PASS (including the e2e tests; do not pass `-short`).

- [x] **Step 4: CI task**

Run: `mise exec -- task ci`
Expected: PASS (golangci-lint clean + tests). If the linter flags the `#nosec G304` fallback read in `httpSignatureScan`, confirm the comment is present exactly as written in Task 3 (it mirrors the existing `signatureMatches` suppression at `internal/link/link.go:243-244`).

- [x] **Step 5: Mark tasks.md complete**

Tick every box in `openspec/changes/add-openapi-handler-linking/tasks.md` (1.1–5.3) now that each is implemented and verified, then commit.

```bash
git add openspec/changes/add-openapi-handler-linking/tasks.md
git commit -m "chore(comet): mark add-openapi-handler-linking tasks complete"
```

---

## Self-Review

**Spec coverage (`specs/ast-linking/spec.md` + `specs/openapi-tools/spec.md` + design + tasks.md):**

- AST-confirmed HTTP handler matching, three-tier ladder → Task 3 (`MatchOpenAPI`/`scoreHTTP`/`classifyHTTPHandler`), Task 4 (tests).
- "operation confirmed by AST handler signature" scenario → Task 4 `TestMatchOpenAPIHighViaAST`, Task 8 e2e.
- "HIGH via string-scan when root absent" → Task 4 `TestMatchOpenAPIHighViaStringScan` (`httpSignatureScan`).
- "MEDIUM via route-registration presence" → Task 4 `TestMatchOpenAPIMediumViaRoute` (`hasRouteRegistration`).
- "LOW for same-named non-handler" → Task 4 `TestMatchOpenAPILowForNonHandler` (AST non-promotion).
- "no operationId or no candidate yields no link" → Task 4 `TestMatchOpenAPINoLink`.
- explain_endpoint returns contract + `implemented_by` (empty list, not error) backward-compatibly → Task 6.
- Store table keyed by (index_id, method, path) + idempotent writer/reader → Tasks 1, 2.
- Ingest wiring gated on root alongside MatchRPCs → Task 5.
- Stale limitation strings replaced → Task 7.
- e2e HTTP implemented_by HIGH + roadmap 0.2 flip → Tasks 8, 9.
- tasks.md groups 1–5 all mapped (1→T1/T2, 2→T3/T4, 3→T5, 4→T6/T7, 5→T8/T9), final verification T10.

**Placeholder scan:** no TBD/TODO/"handle edge cases"; every code step shows full code; commands have expected output.

**Type consistency:** `HTTPOpImplLink{Method,Path,NodeID,Confidence,MatchReason}` is defined in Task 2 and consumed unchanged in Tasks 3/5/6/8. `Op{Method,Path,OperationID}` defined in Task 3, consumed in Task 5 (`collectOps`). `MatchOpenAPI(s, indexID, ops, root) ([]store.HTTPOpImplLink, error)` signature identical across Tasks 3/4/5. `HTTPOpImpl{Symbol,Confidence,Reason}` and `EndpointExplanation{Operation,ImplementedBy}` defined in Task 6, asserted in Tasks 6/8. `tierLabel`/`tierLabel HIGH≥0.85` consistent with the `Confidence < 0.85` HIGH checks used in tests. `scoreHTTP`/`astHTTPHandlerMatch`/`classifyHTTPHandler`/`hasRouteRegistration`/`httpSignatureScan`/`handlerNameCandidates`/`titleFirst`/`isSelector` are all defined once (Task 3) and referenced consistently.

**Note on a flagged verification gap (verify during execution):** Task 5's test constructs a `config.Config`/`config.Repo` literal — confirm the exact field names in `internal/config` before writing the test (the plan assumes `Repo`, `Graph`, `Commit`, `Branch`, `Root`, `OpenAPI`, matching the manifest YAML keys used in `e2e_test.go`). If the in-process `Run` path is awkward to seed, prefer extending the existing `internal/ingest/ingest_test.go` harness rather than introducing a new config-construction pattern.
