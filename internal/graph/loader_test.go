package graph

import (
	"testing"

	"github.com/vend-ai/intel-mcp/internal/store"
)

func TestLoadIsIdempotentAndSkipsMalformed(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	indexID, _ := s.UpsertIndex("org", "svc", "abc", "main", "testdata/sample.json")

	g := &Graph{
		Nodes: []Node{
			{ID: "a", Label: "A", FileType: "code", SourceFile: "a.go"},
			{ID: "", Label: "bad"}, // malformed: no id -> skipped
		},
		Edges: []Edge{{Source: "a", Target: "b", Relation: "calls"}},
	}

	r1, err := Load(s, indexID, g)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if r1.Nodes != 1 {
		t.Fatalf("expected 1 node inserted (malformed skipped), got %d", r1.Nodes)
	}

	// Re-load: counts identical, no duplicates.
	if _, err := Load(s, indexID, g); err != nil {
		t.Fatalf("reload: %v", err)
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM nodes WHERE index_id=?`, indexID).Scan(&n)
	if n != 1 {
		t.Fatalf("expected 1 node after reload, got %d", n)
	}
}
