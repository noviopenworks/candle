package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vend-ai/intel-mcp/internal/config"
	"github.com/vend-ai/intel-mcp/internal/store"
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
