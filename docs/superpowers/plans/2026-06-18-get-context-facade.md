---
change: add-get-context-facade
design-doc: docs/superpowers/specs/2026-06-18-get-context-facade-design.md
base-ref: 3f87c8a97f10f8258d4ba4ea5e0a672d95d5b5df
archived-with: 2026-06-18-add-get-context-facade
---

# get_context Retrieval Facade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `get_context` as the primary Context7-style MCP retrieval tool: a repo-scoped facade with an overview catalog mode and a topic-retrieval mode over code, OpenAPI, protobuf, and private Go libraries.

**Architecture:** A pure, SDK-free `Tools.GetContext(GetContextArgs) (ContextResult, error)` in a new `internal/mcp/context_tools.go`, composing existing store/Tools queries, registered via a thin `registerGetContext` in `server.go`. Topic mode adds codegraph-style one-hop callers/callees for code matches.

**Tech Stack:** Go 1.26, `internal/mcp`, `internal/store`, SQLite-backed query helpers, MCP Go SDK (`mcpsdk`) registration.

## Global Constraints

- Go module path: `github.com/noviopenworks/candle` (use in test imports).
- Additive only: do NOT change existing tool behavior, `internal/store`, parsers, or the registry.
- Repo field on the result MUST be the typed `RepoSummary` struct (design D3) — not `any`.
- `mode:"overview"` returns the catalog only and suppresses topic matches even when a topic is given (design D6).
- Proto RPC package MUST be derived from `ProtoRPC.FullName` (there is no `Package` field on `ProtoRPC`/`ProtoRPCResult`).
- Verification gates: `go test ./...` and `go vet ./...` must pass.

archived-with: 2026-06-18-add-get-context-facade
---

### Task 1: Overview mode (types + catalog)

**Files:**
- Create: `internal/mcp/context_tools.go`
- Test: `internal/mcp/context_tools_test.go`

**Interfaces:**
- Consumes (existing): `Tools.reg.Resolve(repo) (registry.RepoInfo, bool, error)`; `registry.RepoInfo{IndexID int64; Repo, Branch, Commit string}`; `Tools.s.ListAPISpecs(indexID)`, `Tools.s.ListProtoFiles(indexID)`, `Tools.s.FindPrivateLibraries(indexID, "")`, `Tools.s.FindPrivateDeps(indexID, "")`; `Tools.s.DB`; `ErrNotFound`.
- Produces (for later tasks): `GetContextArgs`, `ContextResult`, `RepoSummary`, `ContextCapabilities`, `CapabilitySummary`, `ContextMatches`, `CodeContext`, `ToolHint`, `ResourceScheme`; `func (t *Tools) GetContext(GetContextArgs) (ContextResult, error)`.

- [x] **Step 1: Write the failing overview test**

Create `internal/mcp/context_tools_test.go`:

```go
package mcp

import (
	"testing"

	"github.com/noviopenworks/candle/internal/store"
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
	if _, err = s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location) VALUES(?,?,?,?,?,?)`,
		id, "handler_reserve", "ReserveProduct", "code", "internal/http/reservation_handler.go", "L10"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location) VALUES(?,?,?,?,?,?)`,
		id, "service_reserve", "ReserveService", "code", "internal/reservation/service.go", "L20"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`INSERT INTO edges(index_id,source,target,relation) VALUES(?,?,?,?)`, id, "handler_reserve", "service_reserve", "calls"); err != nil {
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
			RPCs:    []store.ProtoRPC{{Name: "ReserveProduct", FullName: "inventory.v1.InventoryService.ReserveProduct", RequestMessage: "inventory.v1.ReserveProductRequest", ResponseMessage: "inventory.v1.ReserveProductResponse", StreamKind: "unary"}},
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
	if len(out.Limitations) == 0 {
		t.Fatalf("expected limitations")
	}
}
```

- [x] **Step 2: Run the test, verify it fails**

Run: `go test ./internal/mcp -run TestGetContextOverview -v`
Expected: FAIL — undefined `GetContextArgs` / `Tools.GetContext`.

- [x] **Step 3: Implement types + overview**

Create `internal/mcp/context_tools.go`:

```go
package mcp

import (
	"fmt"
	"strings"

	"github.com/noviopenworks/candle/internal/store"
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

// RepoSummary is the typed repo identity exposed by get_context (design D3).
type RepoSummary struct {
	Repo      string `json:"repo"`
	Commit    string `json:"commit"`
	Branch    string `json:"branch"`
	NodeCount int    `json:"node_count"`
}

type ContextResult struct {
	Repo               RepoSummary         `json:"repo"`
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
	CodeSymbols      []CodeContext    `json:"code_symbols,omitempty"`
	Endpoints        any              `json:"endpoints,omitempty"`
	Schemas          []SchemaInfo     `json:"schemas,omitempty"`
	RPCs             []RPCExplanation `json:"rpcs,omitempty"`
	PrivateLibraries any              `json:"private_libraries,omitempty"`
}

type CodeContext struct {
	Node     store.NodeRow   `json:"node"`
	Callers  []store.EdgeRow `json:"callers"`
	Callees  []store.EdgeRow `json:"callees"`
	Resource string          `json:"resource,omitempty"`
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
		Repo: RepoSummary{
			Repo:      ri.Repo,
			Commit:    ri.Commit,
			Branch:    ri.Branch,
			NodeCount: t.nodeCount(ri.IndexID),
		},
		Topic:           args.Topic,
		Mode:            mode,
		Limitations:     contextLimitations(),
		Capabilities:    t.contextCapabilities(ri.IndexID),
		SuggestedNextCalls: overviewHints(ri.Repo),
		ResourceSchemes: contextResourceSchemes(),
	}
	return out, nil
}

// normalizeContextMode keeps overview distinct from all (design D6): unknown/empty -> all.
func normalizeContextMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "overview", "all", "code", "api", "proto", "library":
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
		CodeGraph:        CapabilitySummary{Available: true, Count: t.nodeCount(indexID), Tools: []string{"query_repo", "explain_symbol", "get_file_context"}},
		OpenAPI:          CapabilitySummary{Available: len(apis) > 0, Count: len(apis), Tools: []string{"list_apis", "find_endpoint", "explain_endpoint", "find_schema"}},
		Protobuf:         CapabilitySummary{Available: len(protos) > 0, Count: len(protos), Tools: []string{"list_apis", "find_rpc", "explain_rpc", "find_schema"}},
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
		"Graph traversal is one hop; depth > 1 is deferred.",
	}
}

func graphNodeResource(repo, commit, nodeID string) string {
	return fmt.Sprintf("graph://%s/commit/%s/node/%s", repo, commitOrLatest(commit), nodeID)
}

func commitOrLatest(commit string) string {
	if commit == "" {
		return "latest"
	}
	return commit
}

// rpcPackage derives the proto package from an RPC full name
// ("pkg.Service.Rpc" -> "pkg"); ProtoRPC has no Package field.
func rpcPackage(fullName, service, name string) string {
	pkg := strings.TrimSuffix(fullName, "."+service+"."+name)
	if pkg == fullName {
		// Fallback: strip the last two dotted segments.
		parts := strings.Split(fullName, ".")
		if len(parts) > 2 {
			return strings.Join(parts[:len(parts)-2], ".")
		}
	}
	return pkg
}
```

- [x] **Step 4: Run the test, verify it passes**

Run: `go test ./internal/mcp -run TestGetContextOverview -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/mcp/context_tools.go internal/mcp/context_tools_test.go
git commit -m "feat(mcp): add get_context overview mode"
```

archived-with: 2026-06-18-add-get-context-facade
---

### Task 2: Topic retrieval mode

**Files:**
- Modify: `internal/mcp/context_tools.go`
- Test: `internal/mcp/context_tools_test.go`

**Interfaces:**
- Consumes (existing): `Tools.s.NodesByLabel`, `Tools.s.Callers`, `Tools.s.Callees`, `Tools.s.FindOperations`, `Tools.FindSchema`, `Tools.s.FindRPCs`, `Tools.ExplainRPC`, `Tools.FindPrivateLibrary`; `store.ProtoRPCResult{ProtoRPC, Service, ProtoPath}`; `store.HTTPOperation{OperationID, ...}`; `store.PrivateLibraryResult{ModulePath, ...}`.
- Consumes (from Task 1): `rpcPackage`, `commitOrLatest`, `graphNodeResource`, all result types.
- Produces: `func (t *Tools) contextMatches(...)`.

- [x] **Step 1: Write failing topic tests**

Append to `internal/mcp/context_tools_test.go`:

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
	if len(out.Matches.CodeSymbols[0].Callees) != 1 {
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

func TestGetContextOverviewModeSuppressesMatches(t *testing.T) {
	tools := seedContextTools(t)
	out, err := tools.GetContext(GetContextArgs{Repo: "org/inventory", Topic: "ReserveProduct", Mode: "overview"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Matches.CodeSymbols) != 0 || len(out.Matches.Schemas) != 0 || len(out.Matches.RPCs) != 0 || out.Matches.Endpoints != nil || out.Matches.PrivateLibraries != nil {
		t.Fatalf("overview mode must suppress matches: %+v", out.Matches)
	}
	if out.Capabilities.CodeGraph.Count != 2 {
		t.Fatalf("overview mode must still return the catalog")
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

- [x] **Step 2: Run tests, verify topic/code/overview-suppress tests fail**

Run: `go test ./internal/mcp -run 'TestGetContext' -v`
Expected: `TestGetContextTopicSearchesAllSurfaces` and `TestGetContextCodeModeOnlyReturnsCode` FAIL (matches not populated); `TestGetContextOverviewModeSuppressesMatches` and `TestGetContextUnknownRepo` may already pass.

- [x] **Step 3: Wire topic search into GetContext**

In `GetContext`, before `return out, nil`, insert (note: overview mode skips matching per D6):

```go
	if strings.TrimSpace(args.Topic) != "" && mode != "overview" {
		matches, resources, hints := t.contextMatches(ri.IndexID, ri.Repo, ri.Commit, args.Topic, mode, args.IncludeResources)
		out.Matches = matches
		out.Resources = resources
		out.SuggestedNextCalls = append(hints, out.SuggestedNextCalls...)
	}
	return out, nil
```

Then add `contextMatches`:

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
					resources = append(resources, fmt.Sprintf("proto://%s/commit/%s/rpc/%s/%s/%s", repo, commitOrLatest(commit), rpcPackage(rpc.FullName, rpc.Service, rpc.Name), rpc.Service, rpc.Name))
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
```

- [x] **Step 4: Run all get_context tests**

Run: `go test ./internal/mcp -run 'TestGetContext' -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/mcp/context_tools.go internal/mcp/context_tools_test.go
git commit -m "feat(mcp): add get_context topic retrieval and mode filtering"
```

archived-with: 2026-06-18-add-get-context-facade
---

### Task 3: Register the MCP tool

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `internal/mcp/e2e_surface_test.go`

**Interfaces:**
- Consumes: `mcpsdk.AddTool`, `Tools.GetContext`, helpers `textResult`, `mustJSON`, `toolErr`; `context` package.

- [x] **Step 1: Add to ToolNames**

In `internal/mcp/server.go`, in `var ToolNames`, add `"get_context",` immediately after `"resolve_repo",`.

- [x] **Step 2: Register in NewServer**

In `NewServer`, add after `registerResolveRepo(srv, tools)`:

```go
	registerGetContext(srv, tools)
```

- [x] **Step 3: Add registration function**

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

- [x] **Step 4: Update e2e surface comments**

In `internal/mcp/e2e_surface_test.go`, change the two comments referencing "all 13 tools" (≈ lines 32 and 218) to "all 14 tools" / "advertises all 14." The assertion loops over `ToolNames`, so no numeric assertion change is needed — adding `get_context` to `ToolNames` is what extends the checked surface.

- [x] **Step 5: Run MCP tests**

Run: `go test ./internal/mcp -v`
Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add internal/mcp/server.go internal/mcp/e2e_surface_test.go
git commit -m "feat(mcp): register get_context as the 14th tool"
```

archived-with: 2026-06-18-add-get-context-facade
---

### Task 4: Documentation

**Files:**
- Modify: `docs/tools.md`
- Modify: `docs/examples.md`
- Modify: `README.md`

- [x] **Step 1: Update `docs/tools.md`**

Change the tool count to **14 tools** and insert a `get_context` reference section after `resolve_repo`:

```markdown
### `get_context`

Context7-style retrieval entry point. With only `repo`, returns a catalog of what candle knows about that repo. With `topic`, searches code symbols, HTTP endpoints, schemas, RPCs, proto messages, and private libraries in that repo.

| Arg | Type | Description |
|-----|------|-------------|
| `repo` | string | repo identity (`org/name`) |
| `topic` | string | optional symbol / endpoint / RPC / schema / library topic |
| `mode` | string | optional: `overview`, `code`, `api`, `proto`, `library`, `all` (`overview` returns the catalog only and suppresses topic matches) |
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

**Response:** typed repo summary, grouped capabilities, matches, resource URI hints, suggested next tool calls, and explicit limitations.
```

- [x] **Step 2: Update `docs/examples.md`**

Add a first example titled `Start with get_context`:

```json
{"repo": "org/inventory-service"}
{"repo": "org/inventory-service", "topic": "ReserveProduct", "include_resources": true}
```

Explain that clients should call the precise tools (`explain_symbol`, `explain_rpc`, `explain_endpoint`, `find_private_library`) after `get_context` identifies the relevant surface, following the `suggested_next_calls` in the response.

- [x] **Step 3: Update `README.md`**

Change the advertised count from `13 tools` to `14 tools`, and add near the quick start:

```markdown
Agents typically start with `get_context`: call it with a repo for a catalog, or with a repo plus topic for focused Context7-style retrieval.
```

- [x] **Step 4: Verify build is unaffected**

Run: `go test ./...`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add docs/tools.md docs/examples.md README.md
git commit -m "docs: document get_context retrieval facade"
```

archived-with: 2026-06-18-add-get-context-facade
---

### Task 5: Final verification

**Files:** all files touched above.

- [x] **Step 1: Full test suite**

Run: `go test ./...`
Expected: PASS.

- [x] **Step 2: Static checks**

Run: `go vet ./...`
Expected: PASS.

- [x] **Step 3: Inspect diff scope**

Run: `git diff 3f87c8a97f10f8258d4ba4ea5e0a672d95d5b5df --stat`
Expected: only `internal/mcp/context_tools.go`, `internal/mcp/context_tools_test.go`, `internal/mcp/server.go`, `internal/mcp/e2e_surface_test.go`, `docs/tools.md`, `docs/examples.md`, `README.md` (plus OpenSpec/comet artifacts and the plan/design docs).

archived-with: 2026-06-18-add-get-context-facade
---

## Self-Review

- **Spec coverage:** Task 1 → overview catalog + limitations + unknown-repo handling (req: facade tool, overview mode, limitations). Task 2 → topic retrieval, one-hop code context, mode filter, overview-suppresses-matches (req: topic mode, mode filter incl. new D6 scenario). Task 3 → tool advertised in `tools/list` (req: facade tool / advertised). Task 4 → docs. Task 5 → gates.
- **Placeholder scan:** none — all steps carry concrete code/commands.
- **Type consistency:** `RepoSummary` (typed `Repo` field), `CodeContext.Callees []store.EdgeRow` (test reads `.Callees` without assertion), `rpcPackage` derives package from `FullName` (no nonexistent `ProtoRPC.Package`). `normalizeContextMode` keeps `overview` distinct from `all` (D6).
- **Scope check:** single capability; additive; no store/parser/registry changes.
