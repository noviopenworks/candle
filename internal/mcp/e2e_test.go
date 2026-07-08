package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/noviopenworks/candle/internal/store"
)

// TestEndToEndStdio builds the candle binary, ingests a fixture graph via the
// `index` subcommand, then launches `serve` as a real subprocess over the SDK's
// stdio transport. It uses the SDK client (which performs the JSON-RPC
// initialize handshake) to assert:
//   - tools/list advertises every tool name (base + openapi), and
//   - tools/call for list_repos returns the ingested repo, and
//   - tools/call for explain_endpoint returns the indexed operationId.
//
// Using the SDK's own CommandTransport guarantees the newline-delimited JSON-RPC
// framing matches the server's StdioTransport, so the subprocess exchange is
// reliable rather than brittle hand-rolled framing.
func TestEndToEndStdio(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in -short mode")
	}

	tmp := t.TempDir()

	// 1. Write a fixture graph.json and a manifest pointing at it.
	graphPath := filepath.Join(tmp, "graph.json")
	if err := os.WriteFile(graphPath, []byte(`{
		"nodes": [
			{"id":"n1","label":"ReserveProduct","file_type":"code","source_file":"h.go"},
			{"id":"n2","label":"ReserveSvc","file_type":"code","source_file":"s.go"}
		],
		"edges": [
			{"source":"n1","target":"n2","relation":"calls"}
		],
		"hyperedges": []
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write an OpenAPI spec the manifest will reference, exercising the contract layer.
	specPath := filepath.Join(tmp, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(
		"openapi: 3.0.3\n"+
			"info:\n  title: Inventory API\n  version: \"1.4.0\"\n"+
			"paths:\n"+
			"  /x:\n"+
			"    post:\n"+
			"      operationId: reserveProduct\n"+
			"      summary: Reserve product stock\n"+
			"      responses:\n        '200': { description: ok }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(tmp, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(
		"repos:\n  - repo: org/svc\n    graph: "+graphPath+"\n    commit: abc\n    branch: main\n    openapi:\n      - "+specPath+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Build the binary.
	binPath := filepath.Join(tmp, "candle")
	build := exec.Command("go", "build", "-o", binPath, "github.com/noviopenworks/candle/cmd/candle")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	dbPath := filepath.Join(tmp, "intel.db")

	// 3. Ingest via the `index` subcommand.
	idx := exec.Command(binPath, "--db", dbPath, "--config", manifestPath, "index")
	if out, err := idx.CombinedOutput(); err != nil {
		t.Fatalf("index failed: %v\n%s", err, out)
	} else if !strings.Contains(string(out), "indexed=1") {
		t.Fatalf("expected indexed=1, got: %s", out)
	}

	// 4. Launch `serve` as a subprocess over stdio and connect with the SDK client.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "e2e-test", Version: "0.0.0"}, nil)
	transport := &mcpsdk.CommandTransport{
		Command: exec.Command(binPath, "--db", dbPath, "serve"),
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect (initialize) failed: %v", err)
	}
	defer session.Close()

	// 5. tools/list must advertise all five base tools.
	lt, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	got := map[string]bool{}
	for _, tl := range lt.Tools {
		got[tl.Name] = true
	}
	for _, want := range ToolNames {
		if !got[want] {
			t.Fatalf("tools/list missing %q; got %v", want, keys(got))
		}
	}

	// 6. tools/call list_repos must return the ingested repo.
	res, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "list_repos",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("tools/call list_repos failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("list_repos returned IsError: %+v", res.Content)
	}
	body := contentText(res.Content)
	if !strings.Contains(body, "org/svc") {
		t.Fatalf("list_repos did not return ingested repo org/svc; got: %s", body)
	}

	// 7. tools/call explain_endpoint must return the indexed operationId from the
	// OpenAPI spec, proving the contract layer is wired through index → serve.
	epRes, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "explain_endpoint",
		Arguments: map[string]any{"repo": "org/svc", "method": "POST", "path": "/x"},
	})
	if err != nil {
		t.Fatalf("tools/call explain_endpoint failed: %v", err)
	}
	if epRes.IsError {
		t.Fatalf("explain_endpoint returned IsError: %+v", epRes.Content)
	}
	epBody := contentText(epRes.Content)
	if !strings.Contains(epBody, "reserveProduct") {
		t.Fatalf("explain_endpoint did not return operationId reserveProduct; got: %s", epBody)
	}
}

// TestProtoDoesNotRegressHTTP proves that indexing protobuf alongside OpenAPI
// does not change the shape or counts of the OpenAPI/HTTP tool output: list_apis
// and find_schema still surface exactly one OpenAPI result each, plus the new
// proto result, with stable kind discriminators.
func TestProtoDoesNotRegressHTTP(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	// OpenAPI seed.
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

func TestGoDepDoesNotRegressOthers(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "web", "abc", "main", "/g")
	if err := s.ReplaceAPISpecs(id, []store.APISpecBundle{{
		Spec: store.APISpec{Kind: "openapi", Name: "Web API", Version: "1.0", Path: "api/openapi.yaml"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(id, store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "git.acme.local/platform/auth", Version: "v1", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Libraries: []store.PrivateLibraryBundle{{
			Library: store.PrivateLibrary{ModulePath: "git.acme.local/platform/auth"},
			Exports: []store.PrivateExport{{PackagePath: "git.acme.local/platform/auth", Symbol: "NewClient", Kind: "constructor"}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	tools := NewTools(s)

	apis, _ := tools.ListAPIs("acme/web")
	if len(apis) != 1 || apis[0].Kind != "openapi" {
		t.Fatalf("list_apis regressed: %+v", apis)
	}
	libs, _ := tools.FindPrivateLibrary("acme/web", "auth")
	if len(libs) != 1 {
		t.Fatalf("find_private_library: %+v", libs)
	}
}

func contentText(content []mcpsdk.Content) string {
	var b strings.Builder
	for _, c := range content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
