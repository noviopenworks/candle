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
