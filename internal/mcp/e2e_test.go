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

// TestEndToEndStdio builds the intel-mcp binary, ingests a fixture graph via the
// `index` subcommand, then launches `serve` as a real subprocess over the SDK's
// stdio transport. It uses the SDK client (which performs the JSON-RPC
// initialize handshake) to assert:
//   - tools/list advertises all five base tool names, and
//   - tools/call for list_repos returns the ingested repo.
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

	manifestPath := filepath.Join(tmp, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(
		"repos:\n  - repo: org/svc\n    graph: "+graphPath+"\n    commit: abc\n    branch: main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Build the binary.
	binPath := filepath.Join(tmp, "intel-mcp")
	build := exec.Command("go", "build", "-o", binPath, "github.com/vend-ai/intel-mcp/cmd/intel-mcp")
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
