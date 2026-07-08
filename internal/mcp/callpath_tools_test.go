package mcp

import (
	"errors"
	"testing"

	"github.com/noviopenworks/candle/internal/store"
)

// seedCallPathTools seeds a 3-node call chain a -> b -> c with a back-edge
// c -> a (a cycle), so multi-hop traversal and cycle-cutting are both testable.
func seedCallPathTools(t *testing.T) *Tools {
	t.Helper()
	s, _ := store.Open(":memory:")
	id, _ := s.UpsertIndex("org", "svc", "abc", "main", "/g")
	for _, n := range []struct{ id, label string }{
		{"a", "Alpha"}, {"b", "Beta"}, {"c", "Gamma"},
	} {
		s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file) VALUES(?,?,?,?,?)`,
			id, n.id, n.label, "code", n.id+".go")
	}
	for _, e := range []struct{ src, dst string }{
		{"a", "b"}, {"b", "c"}, {"c", "a"},
	} {
		s.DB.Exec(`INSERT INTO edges(index_id,source,target,relation) VALUES(?,?,?,?)`, id, e.src, e.dst, "calls")
	}
	return NewTools(s)
}

func TestCallPathCalleesMultiHop(t *testing.T) {
	tl := seedCallPathTools(t)
	hop, err := tl.CallPath("org/svc", "a", 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if hop.Node.NodeID != "a" {
		t.Fatalf("root: %+v", hop.Node)
	}
	if len(hop.Children) != 1 || hop.Children[0].Node.NodeID != "b" {
		t.Fatalf("first hop: %+v", hop.Children)
	}
	b := hop.Children[0]
	if len(b.Children) != 1 || b.Children[0].Node.NodeID != "c" {
		t.Fatalf("second hop: %+v", b.Children)
	}
	// Cycle is cut: c -> a exists, but a is on the current path.
	if len(b.Children[0].Children) != 0 {
		t.Fatalf("cycle not cut: %+v", b.Children[0].Children)
	}
}

func TestCallPathCallers(t *testing.T) {
	tl := seedCallPathTools(t)
	hop, err := tl.CallPath("org/svc", "c", 2, "callers")
	if err != nil {
		t.Fatal(err)
	}
	if hop.Node.NodeID != "c" {
		t.Fatalf("root: %+v", hop.Node)
	}
	if len(hop.Children) != 1 || hop.Children[0].Node.NodeID != "b" {
		t.Fatalf("caller hop: %+v", hop.Children)
	}
	if len(hop.Children[0].Children) != 1 || hop.Children[0].Children[0].Node.NodeID != "a" {
		t.Fatalf("caller second hop: %+v", hop.Children[0].Children)
	}
}

func TestCallPathLabelAndDefaultDepth(t *testing.T) {
	tl := seedCallPathTools(t)
	// symbol by label; depth omitted -> default 1 (no recursion past first hop).
	hop, err := tl.CallPath("org/svc", "Alpha", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if hop.Node.NodeID != "a" {
		t.Fatalf("label resolve: %+v", hop.Node)
	}
	if len(hop.Children) != 1 || hop.Children[0].Node.NodeID != "b" {
		t.Fatalf("default depth=1 first hop: %+v", hop.Children)
	}
	if len(hop.Children[0].Children) != 0 {
		t.Fatalf("default depth should not recurse: %+v", hop.Children[0].Children)
	}
}

func TestCallPathDepthCap(t *testing.T) {
	tl := seedCallPathTools(t)
	// depth far over the cap must not panic or hang; the cycle is cut anyway.
	hop, err := tl.CallPath("org/svc", "a", 99, "")
	if err != nil {
		t.Fatal(err)
	}
	if hop.Node.NodeID != "a" {
		t.Fatalf("root: %+v", hop.Node)
	}
}

func TestCallPathUnknown(t *testing.T) {
	tl := seedCallPathTools(t)
	if _, err := tl.CallPath("org/svc", "nope", 0, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown symbol, got %v", err)
	}
	if _, err := tl.CallPath("org/missing", "a", 0, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing repo, got %v", err)
	}
}
