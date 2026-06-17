package mcp

import (
	"testing"

	"github.com/noviopenworks/candlegraph/internal/store"
)

func seedTools(t *testing.T) *Tools {
	t.Helper()
	s, _ := store.Open(":memory:")
	id, _ := s.UpsertIndex("org", "svc", "abc", "main", "/g")
	s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file) VALUES(?,?,?,?,?)`,
		id, "n1", "ReserveProduct", "code", "h.go")
	s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file) VALUES(?,?,?,?,?)`,
		id, "n2", "ReserveSvc", "code", "s.go")
	s.DB.Exec(`INSERT INTO edges(index_id,source,target,relation) VALUES(?,?,?,?)`, id, "n1", "n2", "calls")
	return NewTools(s)
}

func TestListRepos(t *testing.T) {
	tl := seedTools(t)
	repos, err := tl.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Repo != "org/svc" || repos[0].NodeCount != 2 {
		t.Fatalf("unexpected: %+v", repos)
	}
}

func TestExplainSymbol(t *testing.T) {
	tl := seedTools(t)
	out, err := tl.ExplainSymbol("org/svc", "ReserveProduct")
	if err != nil {
		t.Fatal(err)
	}
	if out.Node.NodeID != "n1" {
		t.Fatalf("unexpected node: %+v", out.Node)
	}
	if len(out.Callees) != 1 || out.Callees[0].Target != "n2" {
		t.Fatalf("expected one callee n2, got %+v", out.Callees)
	}
}

func TestExplainSymbolUnknownReturnsEmpty(t *testing.T) {
	tl := seedTools(t)
	_, err := tl.ExplainSymbol("org/svc", "DoesNotExist")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetFileContext(t *testing.T) {
	tl := seedTools(t)
	syms, err := tl.GetFileContext("org/svc", "h.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 1 || syms[0].NodeID != "n1" {
		t.Fatalf("unexpected file context: %+v", syms)
	}
}
