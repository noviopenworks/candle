package graph

import (
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	f, err := os.Open("testdata/sample.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	g, err := Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(g.Edges))
	}
	if g.Nodes[0].ID != "http_reservation_reserveproduct" {
		t.Fatalf("unexpected node id %q", g.Nodes[0].ID)
	}
}

func TestParseEmpty(t *testing.T) {
	g, err := ParseBytes([]byte(`{"nodes":[],"edges":[]}`))
	if err != nil {
		t.Fatalf("parse empty: %v", err)
	}
	if len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Fatal("expected empty graph")
	}
}
