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
)

// writeFixture writes data to <dir>/<rel>, creating parent dirs.
func writeFixture(t *testing.T, dir, rel, data string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestEndToEndToolSurface builds the binary, indexes a two-repo fixture
// (a consumer repo exercising code graph + OpenAPI + protobuf + AST-linked RPC +
// Go private-library usage, and a provider repo defining the library), then
// drives a real stdio serve subprocess to assert the full tool + resource
// surface end-to-end:
//   - tools/list advertises all 13 tools
//   - code-graph tools (query_repo, explain_symbol, get_file_context)
//   - explain_rpc surfaces the AST-confirmed (HIGH) implemented_by link
//   - find_rpc, find_private_library, find_library_consumers
//   - resources: repo://, openapi://operation, proto://rpc, lib://
//   - resolve_repo fuzzy across repos and the unknown-repo error path
func TestEndToEndToolSurface(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in -short mode")
	}

	tmp := t.TempDir()
	inv := filepath.Join(tmp, "inventory")  // consumer repo
	prov := filepath.Join(tmp, "platform")  // provider repo

	// --- consumer repo: code graph -----------------------------------------
	// Node ids/labels are chosen so the AST linker can confirm the RPC impl:
	// the RegisterInventoryServiceServer node provides the service-registration
	// signal, and the ReserveProduct node's source_file is a real Go method.
	writeFixture(t, inv, "graph.json", `{
		"nodes": [
			{"id":"grpc_server_reserveproduct","label":"ReserveProduct","file_type":"code","source_file":"internal/grpc/server.go","source_location":"L10"},
			{"id":"grpc_register","label":"RegisterInventoryServiceServer","file_type":"code","source_file":"internal/grpc/register.go"},
			{"id":"svc_reserve","label":"ReserveSvc","file_type":"code","source_file":"internal/svc/service.go"}
		],
		"edges": [
			{"source":"grpc_server_reserveproduct","target":"svc_reserve","relation":"calls"}
		],
		"hyperedges": []
	}`)

	// Real Go source under the repo root so go/parser confirms the unary shape.
	writeFixture(t, inv, "internal/grpc/server.go", `package grpc

import "context"

type Server struct{}

func (s *Server) ReserveProduct(ctx context.Context, req *ReserveProductRequest) (*ReserveProductResponse, error) {
	return nil, nil
}

type ReserveProductRequest struct{}
type ReserveProductResponse struct{}
`)

	// Protobuf contract.
	writeFixture(t, inv, "proto/inventory.proto", `syntax = "proto3";
package acme.inventory;
option go_package = "git.acme.local/apps/web/gen";

service InventoryService {
  rpc ReserveProduct(ReserveProductRequest) returns (ReserveProductResponse);
}

message ReserveProductRequest {
  string sku = 1;
  int32 quantity = 2;
}
message ReserveProductResponse { bool reserved = 1; }
`)

	// OpenAPI contract.
	writeFixture(t, inv, "api/openapi.yaml", `openapi: 3.0.3
info:
  title: Inventory API
  version: "1.4.0"
paths:
  /reservations:
    post:
      operationId: reserveProduct
      summary: Reserve product stock
      responses:
        '200': { description: ok }
`)

	// Go module: consumer importing the private library (usages).
	writeFixture(t, inv, "go.mod", `module git.acme.local/apps/web

go 1.26

require git.acme.local/platform/auth v1.2.0
`)
	writeFixture(t, inv, "main.go", `package main

import (
	"fmt"

	"git.acme.local/platform/auth"
)

func main() {
	c := auth.NewClient()
	fmt.Println(c.Verify("x"))
}
`)

	// --- provider repo: defines the private library (exports) --------------
	writeFixture(t, prov, "graph.json", `{
		"nodes": [
			{"id":"auth_newclient","label":"NewClient","file_type":"code","source_file":"auth.go"}
		],
		"edges": [],
		"hyperedges": []
	}`)
	writeFixture(t, prov, "go.mod", `module git.acme.local/platform/auth

go 1.26
`)
	writeFixture(t, prov, "auth.go", `// Package auth provides token helpers.
package auth

// Client is an auth client.
type Client struct{}

// NewClient builds a Client.
func NewClient() *Client { return &Client{} }

// Verify checks a token.
func (c *Client) Verify(token string) bool { return token != "" }
`)

	// --- manifest -----------------------------------------------------------
	manifest := "repos:\n" +
		"  - repo: org/inventory\n" +
		"    graph: " + filepath.Join(inv, "graph.json") + "\n" +
		"    commit: abc\n    branch: main\n" +
		"    root: " + inv + "\n" +
		"    openapi:\n      - " + filepath.Join(inv, "api/openapi.yaml") + "\n" +
		"    proto:\n      roots:\n        - " + filepath.Join(inv, "proto") + "\n" +
		"      files:\n        - inventory.proto\n" +
		"    go:\n      modules:\n        - " + filepath.Join(inv, "go.mod") + "\n" +
		"      private_prefixes:\n        - git.acme.local/\n" +
		"  - repo: org/platform-auth\n" +
		"    graph: " + filepath.Join(prov, "graph.json") + "\n" +
		"    commit: def\n    branch: main\n" +
		"    root: " + prov + "\n" +
		"    go:\n      modules:\n        - " + filepath.Join(prov, "go.mod") + "\n" +
		"      private_prefixes:\n        - git.acme.local/\n"
	manifestPath := filepath.Join(tmp, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// --- build + index ------------------------------------------------------
	binPath := filepath.Join(tmp, "candlegraph")
	if out, err := exec.Command("go", "build", "-o", binPath, "github.com/noviopenworks/candlegraph/cmd/candlegraph").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	dbPath := filepath.Join(tmp, "intel.db")
	if out, err := exec.Command(binPath, "--db", dbPath, "--config", manifestPath, "index").CombinedOutput(); err != nil {
		t.Fatalf("index failed: %v\n%s", err, out)
	} else if !strings.Contains(string(out), "indexed=2") {
		t.Fatalf("expected indexed=2, got: %s", out)
	}

	// --- serve + connect ----------------------------------------------------
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "e2e-surface", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.CommandTransport{Command: exec.Command(binPath, "--db", dbPath, "serve")}, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer session.Close()

	call := func(name string, args map[string]any) string {
		t.Helper()
		res, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s call error: %v", name, err)
		}
		if res.IsError {
			t.Fatalf("%s returned IsError: %s", name, contentText(res.Content))
		}
		return contentText(res.Content)
	}
	mustContain := func(label, body string, subs ...string) {
		t.Helper()
		for _, s := range subs {
			if !strings.Contains(body, s) {
				t.Fatalf("%s: missing %q in: %s", label, s, body)
			}
		}
	}

	// tools/list advertises all 13.
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

	// Code-graph tools.
	mustContain("query_repo", call("query_repo", map[string]any{"repo": "org/inventory", "name": "ReserveProduct"}), "ReserveProduct", "internal/grpc/server.go")
	mustContain("explain_symbol", call("explain_symbol", map[string]any{"repo": "org/inventory", "symbol": "ReserveProduct"}), "ReserveProduct", "svc_reserve")
	mustContain("get_file_context", call("get_file_context", map[string]any{"repo": "org/inventory", "file": "internal/grpc/server.go"}), "ReserveProduct")

	// AST-confirmed RPC implementation (HIGH tier with "ast" reason).
	rpcBody := call("explain_rpc", map[string]any{"repo": "org/inventory", "service": "InventoryService", "rpc": "ReserveProduct"})
	mustContain("explain_rpc", rpcBody, "grpc_server_reserveproduct", "ast", "0.9")

	// find_rpc.
	mustContain("find_rpc", call("find_rpc", map[string]any{"repo": "org/inventory", "query": "ReserveProduct", "stream_kind": ""}), "ReserveProduct")

	// Private-library tools: consumer usage + provider definition.
	mustContain("find_library_consumers", call("find_library_consumers", map[string]any{"repo": "org/inventory", "module": "git.acme.local/platform/auth"}), "v1.2.0", "NewClient")
	mustContain("find_private_library", call("find_private_library", map[string]any{"repo": "org/platform-auth", "query": "auth"}), "git.acme.local/platform/auth")

	// resolve_repo fuzzy.
	mustContain("resolve_repo", call("resolve_repo", map[string]any{"query": "invent"}), "org/inventory")

	// Resources over stdio.
	readRes := func(uri string) string {
		t.Helper()
		rr, err := session.ReadResource(ctx, &mcpsdk.ReadResourceParams{URI: uri})
		if err != nil {
			t.Fatalf("ReadResource %s failed: %v", uri, err)
		}
		var b strings.Builder
		for _, c := range rr.Contents {
			b.WriteString(c.Text)
		}
		return b.String()
	}
	// Single-segment refs route through the SDK template matcher.
	mustContain("repo resource", readRes("repo://org/inventory"), "org/inventory")
	mustContain("openapi resource", readRes("openapi://org/inventory/commit/abc/operation/reserveProduct"), "reserveProduct")

	// KNOWN LIMITATION (characterization guard): the proto/lib resource templates
	// use a single `{ref}`/`{module}` variable, and the SDK's RFC 6570 matching
	// will not expand a variable across "/". So multi-segment refs — proto
	// rpc/service/file and lib module paths (which contain slashes) — are
	// currently unreachable via resources/read even though the server's URI
	// parsers handle them and the design advertises these URIs. They return an
	// error today. When the resource router is fixed to support multi-segment
	// refs, flip these to mustContain assertions.
	mustErr := func(uri string) {
		t.Helper()
		if _, err := session.ReadResource(ctx, &mcpsdk.ReadResourceParams{URI: uri}); err == nil {
			t.Fatalf("expected %q unreachable (known multi-segment limitation), but it resolved", uri)
		}
	}
	mustErr("proto://org/inventory/commit/abc/rpc/acme.inventory/InventoryService/ReserveProduct")
	mustErr("proto://org/inventory/commit/abc/file/proto/inventory.proto")
	mustErr("lib://git.acme.local/platform/auth")

	// Error path: unknown repo → tool-level IsError.
	errRes, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "explain_endpoint", Arguments: map[string]any{"repo": "org/nonexistent", "method": "POST", "path": "/x"},
	})
	if err != nil {
		t.Fatalf("explain_endpoint unknown-repo call error: %v", err)
	}
	if !errRes.IsError {
		t.Fatalf("expected IsError for unknown repo; got: %s", contentText(errRes.Content))
	}
}
