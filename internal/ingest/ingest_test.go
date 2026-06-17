package ingest

import (
	"os"
	"path/filepath"
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
