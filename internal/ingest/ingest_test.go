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
