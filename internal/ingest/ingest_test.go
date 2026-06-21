package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noviopenworks/candlegraph/internal/config"
	"github.com/noviopenworks/candlegraph/internal/store"
)

func TestRunIngestsAndToleratesMissing(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	os.WriteFile(graphPath, []byte(`{"nodes":[{"id":"a","label":"A","file_type":"code"}],"edges":[]}`), 0o644)

	cfg := &config.Config{Repos: []config.RepoConfig{
		{Repo: "org/has-graph", Graph: graphPath, Commit: "c1", Branch: "main"},
		{Repo: "org/missing", Graph: filepath.Join(dir, "nope.json"), Commit: "c2"},
	}}

	s, _ := store.Open(":memory:")
	defer s.Close()

	report, err := Run(s, cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.Indexed != 1 || report.Skipped != 1 {
		t.Fatalf("expected 1 indexed / 1 skipped, got %+v", report)
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&n)
	if n != 1 {
		t.Fatalf("expected 1 node ingested, got %d", n)
	}
}

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

func TestRunIndexesProtos(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	os.WriteFile(graphPath, []byte(`{"nodes":[{"id":"n1","label":"ReserveProduct","source_file":"x.go"}],"edges":[],"hyperedges":[]}`), 0o644)

	roots, err := filepath.Abs("../proto/testdata")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	cfg := &config.Config{Repos: []config.RepoConfig{
		{Repo: "acme/inventory", Graph: graphPath},
	}}
	cfg.Repos[0].Proto.Roots = []string{roots}
	cfg.Repos[0].Proto.Files = []string{"inventory.proto"}

	s, _ := store.Open(":memory:")
	defer s.Close()

	if _, err := Run(s, cfg); err != nil {
		t.Fatalf("run: %v", err)
	}

	// Run created the index with empty commit/branch; UpsertIndex is idempotent
	// and returns the existing id.
	id, err := s.UpsertIndex("acme", "inventory", "", "", graphPath)
	if err != nil {
		t.Fatalf("upsert index: %v", err)
	}

	rpcs, err := s.FindRPCs(id, "reserve", "")
	if err != nil {
		t.Fatalf("find rpcs: %v", err)
	}
	if len(rpcs) != 1 {
		t.Fatalf("expected 1 rpc matching 'reserve', got %d", len(rpcs))
	}

	impls, err := s.ProtoRPCImpls(id, "acme.inventory.InventoryService.ReserveProduct")
	if err != nil {
		t.Fatalf("proto rpc impls: %v", err)
	}
	if len(impls) < 1 {
		t.Fatalf("expected at least 1 impl link, got %d", len(impls))
	}
}

func TestRunIndexesGoDeps(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "g.json")
	if err := os.WriteFile(graphPath, []byte(`{"nodes":[],"edges":[],"hyperedges":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	root, _ := filepath.Abs("../godep/testdata/consumer")
	cfg := &config.Config{Repos: []config.RepoConfig{{Repo: "acme/web", Graph: graphPath}}}
	cfg.Repos[0].Go.Modules = []string{filepath.Join(root, "go.mod")}
	cfg.Repos[0].Go.PrivatePrefixes = []string{"git.acme.local/"}

	s, _ := store.Open(":memory:")
	defer s.Close()
	if _, err := Run(s, cfg); err != nil {
		t.Fatalf("run: %v", err)
	}
	id, _ := s.UpsertIndex("acme", "web", "", "", graphPath)
	usages, err := s.PrivateUsagesByModule(id, "git.acme.local/platform/auth")
	if err != nil || len(usages) == 0 {
		t.Fatalf("usages: %+v err=%v", usages, err)
	}
}

// writeProtoRPCFixture builds a self-contained repo under dir: a proto file
// defining a unary RPC, a Go server file implementing it, and a graph.json whose
// node's source_file points (repo-relative) at that Go file. Returns the graph
// path. The RPC full name is acme.inventory.InventoryService.ReserveProduct.
func writeProtoRPCFixture(t *testing.T, dir string) (graphPath string) {
	t.Helper()
	// Go server implementing ReserveProduct as a unary method.
	srcRel := filepath.Join("internal", "grpc", "server.go")
	srcAbs := filepath.Join(dir, srcRel)
	if err := os.MkdirAll(filepath.Dir(srcAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	code := "package grpc\n" +
		"import \"context\"\n" +
		"type Server struct{}\n" +
		"type pbReq struct{}\n" +
		"type pbResp struct{}\n" +
		"func (s *Server) ReserveProduct(ctx context.Context, req *pbReq) (*pbResp, error) { return nil, nil }\n"
	if err := os.WriteFile(srcAbs, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	// Proto file defining the unary RPC plus the service registration symbol's
	// presence is not required for the AST tier; AST is authoritative on its own.
	protoRel := "inventory.proto"
	proto := "syntax = \"proto3\";\n" +
		"package acme.inventory;\n" +
		"service InventoryService {\n" +
		"  rpc ReserveProduct(ReserveProductRequest) returns (ReserveProductResponse);\n" +
		"}\n" +
		"message ReserveProductRequest { string sku = 1; }\n" +
		"message ReserveProductResponse { bool reserved = 1; }\n"
	if err := os.WriteFile(filepath.Join(dir, protoRel), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	// graph.json: node label matches the RPC name, source_file is repo-relative.
	graphPath = filepath.Join(dir, "graph.json")
	g := `{"nodes":[{"id":"n1","label":"ReserveProduct","file_type":"code","source_file":"internal/grpc/server.go"}],"edges":[],"hyperedges":[]}`
	if err := os.WriteFile(graphPath, []byte(g), 0o644); err != nil {
		t.Fatal(err)
	}
	return graphPath
}

// TestRunLinksRPCWithASTRoot verifies that when a repo's source root is set,
// ingestion resolves the RPC impl link via AST and records the HIGH/"ast" tier.
func TestRunLinksRPCWithASTRoot(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeProtoRPCFixture(t, dir)

	cfg := &config.Config{Repos: []config.RepoConfig{
		{Repo: "acme/inventory", Graph: graphPath, Root: dir},
	}}
	cfg.Repos[0].Proto.Roots = []string{dir}
	cfg.Repos[0].Proto.Files = []string{"inventory.proto"}

	s, _ := store.Open(":memory:")
	defer s.Close()

	rep, err := Run(s, cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if rep.Indexed != 1 {
		t.Fatalf("expected 1 indexed, got %+v", rep)
	}

	id, _ := s.UpsertIndex("acme", "inventory", "", "", graphPath)
	impls, err := s.ProtoRPCImpls(id, "acme.inventory.InventoryService.ReserveProduct")
	if err != nil {
		t.Fatalf("impls: %v", err)
	}
	if len(impls) != 1 {
		t.Fatalf("expected 1 impl link, got %d: %+v", len(impls), impls)
	}
	if impls[0].Confidence < 0.85 {
		t.Fatalf("expected HIGH confidence (>=0.85) from AST, got %.2f (%s)", impls[0].Confidence, impls[0].MatchReason)
	}
	if !strings.Contains(impls[0].MatchReason, "ast") {
		t.Fatalf("expected reason to mention ast, got %q", impls[0].MatchReason)
	}
}

// TestRunLinksRPCWithoutRootDegrades verifies that with root empty the run still
// succeeds and the link degrades off the AST tier (no "ast" in the reason).
func TestRunLinksRPCWithoutRootDegrades(t *testing.T) {
	dir := t.TempDir()
	graphPath := writeProtoRPCFixture(t, dir)

	cfg := &config.Config{Repos: []config.RepoConfig{
		{Repo: "acme/inventory", Graph: graphPath}, // Root empty.
	}}
	cfg.Repos[0].Proto.Roots = []string{dir}
	cfg.Repos[0].Proto.Files = []string{"inventory.proto"}

	s, _ := store.Open(":memory:")
	defer s.Close()

	rep, err := Run(s, cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if rep.Indexed != 1 {
		t.Fatalf("expected 1 indexed, got %+v", rep)
	}

	id, _ := s.UpsertIndex("acme", "inventory", "", "", graphPath)
	impls, err := s.ProtoRPCImpls(id, "acme.inventory.InventoryService.ReserveProduct")
	if err != nil {
		t.Fatalf("impls: %v", err)
	}
	if len(impls) != 1 {
		t.Fatalf("expected 1 impl link, got %d: %+v", len(impls), impls)
	}
	if strings.Contains(impls[0].MatchReason, "ast") {
		t.Fatalf("expected non-AST tier without root, got reason %q", impls[0].MatchReason)
	}
}

// TestIngestLinksHTTPHandler verifies that ingestion runs MatchOpenAPI and
// persists an AST-confirmed HTTP handler link when a source root is set.
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

	cfg := &config.Config{Repos: []config.RepoConfig{{
		Repo: "org/svc", Graph: graphPath, Commit: "abc", Branch: "main", Root: dir,
		OpenAPI: []string{specPath},
	}}}
	s, _ := store.Open(":memory:")
	defer s.Close()
	if _, err := Run(s, cfg); err != nil {
		t.Fatalf("run: %v", err)
	}

	id, _ := s.UpsertIndex("org", "svc", "abc", "main", graphPath)
	impls, err := s.HTTPOpImpls(id, "POST", "/x")
	if err != nil || len(impls) == 0 {
		t.Fatalf("expected HTTP impl link, got %+v err=%v", impls, err)
	}
	if impls[0].NodeID != "h1" || impls[0].Confidence < 0.85 {
		t.Fatalf("expected HIGH link to h1, got %+v", impls)
	}
}

// TestRunToleratesBadOpenAPISpecs verifies that a missing spec file and a
// Swagger 2.0 spec are each skipped with a warning while the repo's graph is
// still indexed and the run does not abort.
func TestRunToleratesBadOpenAPISpecs(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	os.WriteFile(graphPath, []byte(`{"nodes":[{"id":"a","label":"A","file_type":"code"}],"edges":[]}`), 0o644)

	missingSpec := filepath.Join(dir, "does-not-exist.yaml")
	swagger2 := filepath.Join(dir, "swagger2.yaml")
	os.WriteFile(swagger2, []byte("swagger: \"2.0\"\ninfo:\n  title: Legacy\n  version: \"1.0\"\npaths: {}\n"), 0o644)

	cfg := &config.Config{Repos: []config.RepoConfig{
		{Repo: "org/svc", Graph: graphPath, Commit: "c1", Branch: "main", OpenAPI: []string{missingSpec, swagger2}},
	}}
	s, _ := store.Open(":memory:")
	defer s.Close()

	rep, err := Run(s, cfg)
	if err != nil {
		t.Fatalf("run aborted on bad specs: %v", err)
	}
	if rep.Indexed != 1 {
		t.Fatalf("expected repo still indexed despite bad specs, got %+v", rep)
	}
	if len(rep.Warnings) != 2 {
		t.Fatalf("expected 2 warnings (missing + swagger2), got %v", rep.Warnings)
	}
	var ops, specs, nodes int
	s.DB.QueryRow(`SELECT COUNT(*) FROM http_operations`).Scan(&ops)
	s.DB.QueryRow(`SELECT COUNT(*) FROM api_specs`).Scan(&specs)
	s.DB.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&nodes)
	if ops != 0 || specs != 0 {
		t.Fatalf("expected no specs/operations from bad inputs, got specs=%d ops=%d", specs, ops)
	}
	if nodes != 1 {
		t.Fatalf("expected graph still ingested (1 node), got %d", nodes)
	}
}
