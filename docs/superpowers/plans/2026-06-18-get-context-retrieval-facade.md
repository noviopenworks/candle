# get_context Retrieval Facade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `get_context` as the primary Context7-style retrieval tool for repo-scoped capability discovery and topic-oriented context lookup.

**Architecture:** Keep existing precise tools as follow-ups, and add a pure SDK-free `Tools.GetContext` method plus MCP registration. The method composes existing store queries for repo metadata, code graph symbols, OpenAPI, protobuf, and private Go library data; code topics include one-hop callers/callees like a codegraph response.

**Tech Stack:** Go 1.26, `internal/mcp`, `internal/store`, SQLite-backed query helpers, MCP Go SDK registration in `internal/mcp/server.go`.

---

## File Structure

- Create: `internal/mcp/context_tools.go`
  - Owns `GetContext`, input/result structs, overview mode, topic mode, limitations, resources, and suggested next-call hints.
- Create: `internal/mcp/context_tools_test.go`
  - Unit tests for overview mode, topic mode, codegraph-like symbol context, mode filtering, and unknown repos.
- Modify: `internal/mcp/server.go`
  - Add `get_context` to `ToolNames` and register the MCP tool.
- Modify: `internal/mcp/e2e_surface_test.go`
  - Update expected advertised tool count/name list.
- Modify: `docs/tools.md`
  - Document `get_context` request/response shapes and examples.
- Modify: `docs/examples.md`
  - Add recommended Context7-style flow using `get_context` first, then precise follow-up tools.
- Modify: `README.md`
  - Update advertised tool count and mention the retrieval-first entry point.
- Optional after implementation: OpenSpec spec update under `openspec/specs/` if following Comet archive flow.

---

### Task 1: Add Failing Tests For Overview Mode

**Files:**
- Create: `internal/mcp/context_tools_test.go`

- [ ] **Step 1: Write the overview-mode test**

Create `internal/mcp/context_tools_test.go` with:

```go
package mcp

import (
	"testing"

	"github.com/noviopenworks/candlegraph/internal/store"
)

func seedContextTools(t *testing.T) *Tools {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.UpsertIndex("org", "inventory", "abc123", "main", "/graphs/inventory/graph.json")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location) VALUES(?,?,?,?,?,?)`,
		id, "handler_reserve", "ReserveProduct", "code", "internal/http/reservation_handler.go", "L10")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location) VALUES(?,?,?,?,?,?)`,
		id, "service_reserve", "ReserveService", "code", "internal/reservation/service.go", "L20")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.Exec(`INSERT INTO edges(index_id,source,target,relation) VALUES(?,?,?,?)`, id, "handler_reserve", "service_reserve", "calls")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceAPISpecs(id, []store.APISpecBundle{{
		Spec:       store.APISpec{Kind: "openapi", Name: "Inventory API", Version: "1.0.0", Path: "api/openapi.yaml"},
		Operations: []store.HTTPOperation{{Method: "POST", Path: "/v1/reservations", OperationID: "reserveProduct", Summary: "Reserve product", RequestSchema: "ReserveProductRequest", ResponseSchema: "Reservation"}},
		Schemas:    []store.APISchema{{Name: "ReserveProductRequest", Kind: "openapi_schema"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceProtoFiles(id, []store.ProtoFileBundle{{
		File: store.ProtoFile{Path: "proto/inventory.proto", Package: "inventory.v1"},
		Services: []store.ProtoServiceBundle{{
			Service: store.ProtoService{Name: "InventoryService", FullName: "inventory.v1.InventoryService"},
			RPCs: []store.ProtoRPC{{Name: "ReserveProduct", FullName: "inventory.v1.InventoryService.ReserveProduct", RequestMessage: "inventory.v1.ReserveProductRequest", ResponseMessage: "inventory.v1.ReserveProductResponse", StreamKind: "unary"}},
		}},
		Messages: []store.ProtoMessage{{Name: "ReserveProductRequest", FullName: "inventory.v1.ReserveProductRequest"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(id, store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "github.com/org/auth", Version: "v1.2.3", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Usages:       []store.PrivateUsage{{ModulePath: "github.com/org/auth", Version: "v1.2.3", PackagePath: "github.com/org/auth", Symbol: "ValidateToken", File: "internal/http/auth.go", Line: 12}},
	}); err != nil {
		t.Fatal(err)
	}
	return NewTools(s)
}

func TestGetContextOverview(t *testing.T) {
	tools := seedContextTools(t)
	out, err := tools.GetContext(GetContextArgs{Repo: "org/inventory"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Repo.Repo != "org/inventory" || out.Repo.Commit != "abc123" {
		t.Fatalf("repo summary mismatch: %+v", out.Repo)
	}
	if out.Topic != "" {
		t.Fatalf("overview topic should be empty: %q", out.Topic)
	}
	if out.Capabilities.CodeGraph.Count != 2 || !out.Capabilities.CodeGraph.Available {
		t.Fatalf("code graph capability mismatch: %+v", out.Capabilities.CodeGraph)
	}
	if out.Capabilities.OpenAPI.Count != 1 || out.Capabilities.Protobuf.Count != 1 || out.Capabilities.PrivateLibraries.Count != 1 {
		t.Fatalf("capabilities mismatch: %+v", out.Capabilities)
	}
	if len(out.SuggestedNextCalls) == 0 {
		t.Fatalf("expected suggested next calls")
	}
	if len(out.ResourceSchemes) == 0 {
		t.Fatalf("expected resource schemes")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestGetContextOverview -v`

Expected: FAIL with undefined `GetContextArgs` or missing `Tools.GetContext`.

---

### Task 2: Implement Overview Mode

**Files:**
- Create: `internal/mcp/context_tools.go`

- [ ] **Step 1: Add the minimal implementation**

Create `internal/mcp/context_tools.go` with:

```go
package mcp

import (
	"fmt"
	"strings"
)

// GetContextArgs is the pure-tool input for get_context. Repo is required.
// Topic is optional: empty means repo catalog, non-empty means focused lookup.
type GetContextArgs struct {
	Repo             string `json:"repo" jsonschema:"repo identity (org/name)"`
	Topic            string `json:"topic,omitempty" jsonschema:"optional symbol/API/schema/library topic"`
	Mode             string `json:"mode,omitempty" jsonschema:"optional: overview|code|api|proto|library|all"`
	Depth            int    `json:"depth,omitempty" jsonschema:"optional graph depth; v1 supports 1"`
	IncludeResources bool   `json:"include_resources,omitempty" jsonschema:"include resource URI hints"`
}

type ContextResult struct {
	Repo               any                 `json:"repo"`
	Topic              string              `json:"topic,omitempty"`
	Mode               string              `json:"mode"`
	Capabilities       ContextCapabilities `json:"capabilities"`
	Matches            ContextMatches      `json:"matches,omitempty"`
	SuggestedNextCalls []ToolHint          `json:"suggested_next_calls"`
	ResourceSchemes    []ResourceScheme    `json:"resource_schemes,omitempty"`
	Resources          []string            `json:"resources,omitempty"`
	Limitations        []string            `json:"limitations"`
}

type ContextCapabilities struct {
	CodeGraph        CapabilitySummary `json:"code_graph"`
	OpenAPI          CapabilitySummary `json:"openapi"`
	Protobuf         CapabilitySummary `json:"protobuf"`
	PrivateLibraries CapabilitySummary `json:"private_libraries"`
}

type CapabilitySummary struct {
	Available bool     `json:"available"`
	Count     int      `json:"count"`
	Tools     []string `json:"tools"`
}

type ContextMatches struct {
	CodeSymbols      []CodeContext       `json:"code_symbols,omitempty"`
	Endpoints        any                 `json:"endpoints,omitempty"`
	Schemas          []SchemaInfo        `json:"schemas,omitempty"`
	RPCs             []RPCExplanation    `json:"rpcs,omitempty"`
	PrivateLibraries any                 `json:"private_libraries,omitempty"`
}

type CodeContext struct {
	Node     any      `json:"node"`
	Callers  any      `json:"callers"`
	Callees  any      `json:"callees"`
	Resource string   `json:"resource,omitempty"`
}

type ToolHint struct {
	Tool   string         `json:"tool"`
	Reason string         `json:"reason"`
	Args   map[string]any `json:"args,omitempty"`
}

type ResourceScheme struct {
	Scheme      string `json:"scheme"`
	Description string `json:"description"`
}

// GetContext implements get_context as a repo-scoped retrieval facade.
func (t *Tools) GetContext(args GetContextArgs) (ContextResult, error) {
	ri, ok, err := t.reg.Resolve(args.Repo)
	if err != nil {
		return ContextResult{}, err
	}
	if !ok {
		return ContextResult{}, ErrNotFound
	}
	mode := normalizeContextMode(args.Mode)
	out := ContextResult{
		Repo:         ri,
		Topic:        args.Topic,
		Mode:         mode,
		Limitations:  contextLimitations(),
	}
	out.Capabilities = t.contextCapabilities(ri.IndexID)
	out.SuggestedNextCalls = overviewHints(ri.Repo)
	out.ResourceSchemes = contextResourceSchemes()
	return out, nil
}

func normalizeContextMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "overview", "all":
		return "all"
	case "code", "api", "proto", "library":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "all"
	}
}

func (t *Tools) contextCapabilities(indexID int64) ContextCapabilities {
	apis, _ := t.s.ListAPISpecs(indexID)
	protos, _ := t.s.ListProtoFiles(indexID)
	libs, _ := t.s.FindPrivateLibraries(indexID, "")
	deps, _ := t.s.FindPrivateDeps(indexID, "")
	return ContextCapabilities{
		CodeGraph: CapabilitySummary{Available: true, Count: t.nodeCount(indexID), Tools: []string{"query_repo", "explain_symbol", "get_file_context"}},
		OpenAPI: CapabilitySummary{Available: len(apis) > 0, Count: len(apis), Tools: []string{"list_apis", "find_endpoint", "explain_endpoint", "find_schema"}},
		Protobuf: CapabilitySummary{Available: len(protos) > 0, Count: len(protos), Tools: []string{"list_apis", "find_rpc", "explain_rpc", "find_schema"}},
		PrivateLibraries: CapabilitySummary{Available: len(libs)+len(deps) > 0, Count: len(libs) + len(deps), Tools: []string{"find_private_library", "find_library_consumers"}},
	}
}

func (t *Tools) nodeCount(indexID int64) int {
	rows, err := t.s.DB.Query(`SELECT COUNT(*) FROM nodes WHERE index_id=?`, indexID)
	if err != nil {
		return 0
	}
	defer rows.Close()
	var n int
	if rows.Next() {
		_ = rows.Scan(&n)
	}
	return n
}

func overviewHints(repo string) []ToolHint {
	return []ToolHint{
		{Tool: "get_context", Reason: "retrieve focused context by topic", Args: map[string]any{"repo": repo, "topic": "<symbol endpoint rpc schema or library>"}},
		{Tool: "explain_symbol", Reason: "walk codegraph callers and callees", Args: map[string]any{"repo": repo, "symbol": "<symbol>"}},
		{Tool: "find_endpoint", Reason: "search HTTP endpoints", Args: map[string]any{"repo": repo, "query": "<path operationId or summary>"}},
		{Tool: "find_rpc", Reason: "search protobuf RPCs", Args: map[string]any{"repo": repo, "query": "<service or rpc>"}},
		{Tool: "find_private_library", Reason: "search internal Go modules", Args: map[string]any{"repo": repo, "query": "<module or purpose>"}},
	}
}

func contextResourceSchemes() []ResourceScheme {
	return []ResourceScheme{
		{Scheme: "repo://org/repo", Description: "repo snapshot summary"},
		{Scheme: "graph://org/repo/commit/<sha>/node/<nodeID>", Description: "code graph node"},
		{Scheme: "openapi://org/repo/commit/<sha>/operation/<operationId>", Description: "OpenAPI operation"},
		{Scheme: "openapi://org/repo/commit/<sha>/schema/<schemaName>", Description: "OpenAPI schema"},
		{Scheme: "proto://org/repo/commit/<sha>/rpc/<package>/<service>/<rpc>", Description: "protobuf RPC"},
		{Scheme: "proto://org/repo/commit/<sha>/message/<package>/<message>", Description: "protobuf message"},
		{Scheme: "lib://<module-path>", Description: "private library"},
	}
}

func contextLimitations() []string {
	return []string{
		"OpenAPI endpoint implementation linking is not yet available in get_context v1.",
		"Cross-repo RPC consumed_by aggregation is deferred.",
		"Cross-repo private library consumer aggregation is deferred.",
	}
}

func graphNodeResource(repo, commit, nodeID string) string {
	if commit == "" {
		commit = "latest"
	}
	return fmt.Sprintf("graph://%s/commit/%s/node/%s", repo, commit, nodeID)
}
```

- [ ] **Step 2: Run overview test**

Run: `go test ./internal/mcp -run TestGetContextOverview -v`

Expected: PASS.

---

### Task 3: Add Topic Retrieval Tests

**Files:**
- Modify: `internal/mcp/context_tools_test.go`

- [ ] **Step 1: Add tests for topic mode and mode filtering**

Append:

```go
func TestGetContextTopicSearchesAllSurfaces(t *testing.T) {
	tools := seedContextTools(t)
	out, err := tools.GetContext(GetContextArgs{Repo: "org/inventory", Topic: "ReserveProduct", IncludeResources: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.Topic != "ReserveProduct" {
		t.Fatalf("topic mismatch: %q", out.Topic)
	}
	if len(out.Matches.CodeSymbols) != 1 {
		t.Fatalf("expected one code symbol match, got %+v", out.Matches.CodeSymbols)
	}
	if len(out.Matches.CodeSymbols[0].Callees.([]store.EdgeRow)) != 1 {
		t.Fatalf("expected codegraph-like callees: %+v", out.Matches.CodeSymbols[0])
	}
	if len(out.Matches.Schemas) == 0 {
		t.Fatalf("expected schema match")
	}
	if len(out.Matches.RPCs) == 0 {
		t.Fatalf("expected rpc match")
	}
	if len(out.Resources) == 0 {
		t.Fatalf("expected resource URIs")
	}
}

func TestGetContextCodeModeOnlyReturnsCode(t *testing.T) {
	tools := seedContextTools(t)
	out, err := tools.GetContext(GetContextArgs{Repo: "org/inventory", Topic: "ReserveProduct", Mode: "code"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Matches.CodeSymbols) != 1 {
		t.Fatalf("expected code symbol match")
	}
	if len(out.Matches.Schemas) != 0 || len(out.Matches.RPCs) != 0 || out.Matches.Endpoints != nil {
		t.Fatalf("code mode should not include non-code matches: %+v", out.Matches)
	}
}

func TestGetContextUnknownRepo(t *testing.T) {
	tools := seedContextTools(t)
	_, err := tools.GetContext(GetContextArgs{Repo: "org/missing"})
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/mcp -run 'TestGetContext(Topic|CodeMode|UnknownRepo)' -v`

Expected: topic and code-mode tests FAIL because matches are not populated yet; unknown repo may already pass.

---

### Task 4: Implement Topic Retrieval Mode

**Files:**
- Modify: `internal/mcp/context_tools.go`

- [ ] **Step 1: Add topic retrieval inside `GetContext`**

Replace the tail of `GetContext` after `out.ResourceSchemes = contextResourceSchemes()` with:

```go
	out.ResourceSchemes = contextResourceSchemes()
	if strings.TrimSpace(args.Topic) != "" {
		matches, resources, hints := t.contextMatches(ri.IndexID, ri.Repo, ri.Commit, args.Topic, mode, args.IncludeResources)
		out.Matches = matches
		out.Resources = resources
		out.SuggestedNextCalls = append(hints, out.SuggestedNextCalls...)
	}
	return out, nil
```

Then add:

```go
func (t *Tools) contextMatches(indexID int64, repo, commit, topic, mode string, includeResources bool) (ContextMatches, []string, []ToolHint) {
	var matches ContextMatches
	var resources []string
	var hints []ToolHint
	include := func(surface string) bool { return mode == "all" || mode == surface }

	if include("code") {
		nodes, _ := t.s.NodesByLabel(indexID, topic)
		for _, n := range nodes {
			callers, _ := t.s.Callers(indexID, n.NodeID)
			callees, _ := t.s.Callees(indexID, n.NodeID)
			cc := CodeContext{Node: n, Callers: callers, Callees: callees}
			if includeResources {
				cc.Resource = graphNodeResource(repo, commit, n.NodeID)
				resources = append(resources, cc.Resource)
			}
			matches.CodeSymbols = append(matches.CodeSymbols, cc)
		}
		if len(matches.CodeSymbols) > 0 {
			hints = append(hints, ToolHint{Tool: "explain_symbol", Reason: "code symbol matched topic", Args: map[string]any{"repo": repo, "symbol": topic}})
		}
	}

	if include("api") {
		ops, _ := t.s.FindOperations(indexID, topic)
		if len(ops) > 0 {
			matches.Endpoints = ops
			hints = append(hints, ToolHint{Tool: "find_endpoint", Reason: "HTTP operation matched topic", Args: map[string]any{"repo": repo, "query": topic}})
			if includeResources {
				for _, op := range ops {
					if op.OperationID != "" {
						resources = append(resources, fmt.Sprintf("openapi://%s/commit/%s/operation/%s", repo, commitOrLatest(commit), op.OperationID))
					}
				}
			}
		}
	}

	if include("api") || include("proto") {
		schemas, _ := t.FindSchema(repo, topic)
		matches.Schemas = append(matches.Schemas, schemas...)
		if len(schemas) > 0 {
			hints = append(hints, ToolHint{Tool: "find_schema", Reason: "schema/message matched topic", Args: map[string]any{"repo": repo, "query": topic}})
		}
	}

	if include("proto") {
		rpcs, _ := t.s.FindRPCs(indexID, topic, "")
		for _, rpc := range rpcs {
			expl, err := t.ExplainRPC(repo, rpc.Service, rpc.Name)
			if err == nil {
				matches.RPCs = append(matches.RPCs, expl)
				if includeResources {
					resources = append(resources, fmt.Sprintf("proto://%s/commit/%s/rpc/%s/%s/%s", repo, commitOrLatest(commit), rpc.Package, rpc.Service, rpc.Name))
				}
			}
		}
		if len(matches.RPCs) > 0 {
			hints = append(hints, ToolHint{Tool: "find_rpc", Reason: "RPC matched topic", Args: map[string]any{"repo": repo, "query": topic}})
		}
	}

	if include("library") {
		libs, _ := t.FindPrivateLibrary(repo, topic)
		if len(libs) > 0 {
			matches.PrivateLibraries = libs
			hints = append(hints, ToolHint{Tool: "find_private_library", Reason: "private library matched topic", Args: map[string]any{"repo": repo, "query": topic}})
			if includeResources {
				for _, lib := range libs {
					resources = append(resources, "lib://"+lib.ModulePath)
				}
			}
		}
	}

	return matches, resources, hints
}

func commitOrLatest(commit string) string {
	if commit == "" {
		return "latest"
	}
	return commit
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/mcp -run 'TestGetContext' -v`

Expected: PASS.

---

### Task 5: Register MCP Tool

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `internal/mcp/e2e_surface_test.go`

- [ ] **Step 1: Add `get_context` to `ToolNames`**

In `internal/mcp/server.go`, add `"get_context"` after `"resolve_repo"` in `ToolNames`.

- [ ] **Step 2: Register the tool in `NewServer`**

Add after `registerResolveRepo(srv, tools)`:

```go
	registerGetContext(srv, tools)
```

- [ ] **Step 3: Add registration function**

Add after `registerResolveRepo`:

```go
func registerGetContext(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_context",
		Description: "Context7-style retrieval entry point: repo catalog or topic context across code, APIs, protobuf, and private libraries.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args GetContextArgs) (*mcpsdk.CallToolResult, any, error) {
		out, err := tools.GetContext(args)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(out)), nil, nil
	})
}
```

- [ ] **Step 4: Update e2e surface expectation**

Open `internal/mcp/e2e_surface_test.go` and update any expected count/list from 13 tools to 14 tools, including `get_context` in the same order as `ToolNames`.

- [ ] **Step 5: Run MCP tests**

Run: `go test ./internal/mcp -v`

Expected: PASS.

---

### Task 6: Document The Tool API

**Files:**
- Modify: `docs/tools.md`
- Modify: `docs/examples.md`
- Modify: `README.md`

- [ ] **Step 1: Update tool count and list in `docs/tools.md`**

Change the first section to say **14 tools** and insert `get_context` after `resolve_repo`.

- [ ] **Step 2: Add `get_context` reference section**

Add after `resolve_repo`:

```markdown
### `get_context`

Context7-style retrieval entry point. With only `repo`, returns a catalog of what candlegraph knows about that repo. With `topic`, searches code symbols, HTTP endpoints, schemas, RPCs, proto messages, and private libraries in that repo.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity (`org/name`) |
| `topic` | string | optional symbol / endpoint / RPC / schema / library topic |
| `mode` | string | optional: `overview`, `code`, `api`, `proto`, `library`, `all` |
| `depth` | number | optional; v1 supports one-hop code context |
| `include_resources` | boolean | include exact resource URI hints |

**Overview request:**

```json
{"repo": "org/inventory-service"}
```

**Topic request:**

```json
{"repo": "org/inventory-service", "topic": "ReserveProduct", "include_resources": true}
```

**Response:** grouped capabilities, matches, resource URI hints, suggested next tool calls, and explicit limitations.
```

- [ ] **Step 3: Update `docs/examples.md`**

Add a first example titled `Start with get_context` showing:

```json
{"repo": "org/inventory-service"}
{"repo": "org/inventory-service", "topic": "ReserveProduct", "include_resources": true}
```

Explain that clients should call precise tools (`explain_symbol`, `explain_rpc`, `explain_endpoint`, `find_private_library`) after `get_context` identifies the relevant surface.

- [ ] **Step 4: Update `README.md`**

Change `13 tools` to `14 tools`, and add a short sentence near the quick start:

```markdown
Agents typically start with `get_context`: call it with a repo for a catalog, or with a repo plus topic for focused Context7-style retrieval.
```

- [ ] **Step 5: Run documentation sanity checks**

Run: `go test ./...`

Expected: PASS.

---

### Task 7: Final Verification

**Files:**
- All files touched by previous tasks.

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Run static checks**

Run: `go vet ./...`

Expected: PASS.

- [ ] **Step 3: Inspect diff**

Run: `git diff -- internal/mcp/context_tools.go internal/mcp/context_tools_test.go internal/mcp/server.go internal/mcp/e2e_surface_test.go docs/tools.md docs/examples.md README.md`

Expected: diff only contains `get_context` implementation, tests, MCP registration, and docs updates.

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/context_tools.go internal/mcp/context_tools_test.go internal/mcp/server.go internal/mcp/e2e_surface_test.go docs/tools.md docs/examples.md README.md
git commit -m "feat(mcp): add get_context retrieval facade"
```

---

## Self-Review

- Spec coverage: The plan implements repo catalog mode, topic retrieval mode, codegraph-like one-hop code context, resource URI hints, tool registration, and docs.
- Placeholder scan: No placeholder implementation steps are left; deferred product limitations are explicit runtime strings.
- Type consistency: `GetContextArgs`, `ContextResult`, `ContextCapabilities`, `ContextMatches`, `CodeContext`, `ToolHint`, and `ResourceScheme` are defined before use.
- Scope check: This plan intentionally does not implement OpenAPI handler linking, cross-repo RPC consumers, cross-repo library aggregation, embeddings, or multi-hop traversal.
