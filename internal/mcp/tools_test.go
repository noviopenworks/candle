package mcp

import (
	"context"
	"testing"

	"github.com/noviopenworks/candlegraph/internal/store"
)

var _ func(context.Context, *store.Store, map[int64]bool) error = ServeScoped

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

func TestNewToolsScopedFiltersListAndResolve(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	allowedID, err := s.UpsertIndex("org", "allowed", "a1", "main", "/g/allowed.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertIndex("org", "hidden", "h1", "main", "/g/hidden.json"); err != nil {
		t.Fatal(err)
	}

	tl := NewToolsScoped(s, map[int64]bool{allowedID: true})
	repos, err := tl.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Repo != "org/allowed" {
		t.Fatalf("scoped ListRepos should return only org/allowed, got %+v", repos)
	}
	best, candidates, err := tl.ResolveRepo("org/hidden")
	if err != nil {
		t.Fatal(err)
	}
	if best != nil || len(candidates) != 0 {
		t.Fatalf("scoped ResolveRepo should hide org/hidden, best=%+v candidates=%+v", best, candidates)
	}
	if srv := NewServerScoped(s, map[int64]bool{allowedID: true}); srv == nil {
		t.Fatal("NewServerScoped returned nil")
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
