package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/noviopenworks/candle/internal/store"
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
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestNotFoundCarriesReason locks in roadmap 1.2: a not-found result tells the
// agent *why* (repo vs symbol), not just a bare "not found".
func TestNotFoundCarriesReason(t *testing.T) {
	tl := seedTools(t)

	_, err := tl.ExplainSymbol("org/missing", "x")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "org/missing") {
		t.Fatalf("repo-not-found reason should mention the repo, got %q", err.Error())
	}

	_, err = tl.ExplainSymbol("org/svc", "DoesNotExist")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "DoesNotExist") || !strings.Contains(err.Error(), "symbol") {
		t.Fatalf("symbol-not-found reason should mention the symbol, got %q", err.Error())
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

func TestQueryRepoWithSourcePreservesDefaultShape(t *testing.T) {
	tools := seedSourceContentTools(t)
	out, err := tools.QueryRepoWithSource(QueryRepoArgs{Repo: "org/repo", Name: "ReserveProduct"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.([]store.NodeRow); !ok {
		t.Fatalf("default query_repo shape = %T, want []store.NodeRow", out)
	}
}

func TestQueryRepoWithSourceHydratesExplicitSnippet(t *testing.T) {
	tools := seedSourceContentTools(t)
	tools.sourceHydrator = testHydrator("line1\nline2\nline3\n", "text/plain")
	out, err := tools.QueryRepoWithSource(QueryRepoArgs{
		Repo:          "org/repo",
		Name:          "ReserveProduct",
		SourceContent: &SourceContentOptions{Mode: sourceContentModeSnippet, LineRadius: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.([]SourceNodeResult)
	if !ok {
		t.Fatalf("hydrated query_repo shape = %T, want []SourceNodeResult", out)
	}
	if len(got) != 1 || got[0].Node.NodeID != "n1" || got[0].SourceContent.Status != sourceContentStatusFetched {
		t.Fatalf("hydrated query_repo mismatch: %+v", got)
	}
}

// TestQueryRepoWithSourceKeepsNodesPastCandidateLimit proves that enabling
// source_content never hides structural matches: every matched node is returned
// even when there are more nodes than max_candidates, and the overflow nodes
// carry a "skipped" envelope rather than being dropped.
func TestQueryRepoWithSourceKeepsNodesPastCandidateLimit(t *testing.T) {
	tools := seedSourceContentTools(t)
	if _, err := tools.s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location,source_url) VALUES(1,?,?,?,?,?,?)`,
		"n2", "ReserveProduct", "code", "internal/handler.go", "L5", "https://raw.githubusercontent.com/org/repo/abc/internal/handler.go"); err != nil {
		t.Fatal(err)
	}
	tools.sourceHydrator = testHydrator("line1\nline2\nline3\n", "text/plain")
	out, err := tools.QueryRepoWithSource(QueryRepoArgs{
		Repo:          "org/repo",
		Name:          "ReserveProduct",
		SourceContent: &SourceContentOptions{Mode: sourceContentModeFull, MaxCandidates: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.([]SourceNodeResult)
	if !ok {
		t.Fatalf("hydrated query_repo shape = %T, want []SourceNodeResult", out)
	}
	if len(got) != 2 {
		t.Fatalf("expected all 2 matched nodes returned, got %d: %+v", len(got), got)
	}
	if got[0].SourceContent.Status != sourceContentStatusFetched {
		t.Fatalf("first node should be hydrated: %+v", got[0])
	}
	if got[1].SourceContent.Status != sourceContentStatusSkipped {
		t.Fatalf("overflow node should be skipped, not dropped: %+v", got[1])
	}
}

func TestExplainSymbolWithSourcePreservesDefaultShape(t *testing.T) {
	tools := seedSourceContentTools(t)
	out, err := tools.ExplainSymbolWithSource(ExplainSymbolArgs{Repo: "org/repo", Symbol: "ReserveProduct"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.(SymbolExplanation); !ok {
		t.Fatalf("default explain_symbol shape = %T, want SymbolExplanation", out)
	}
}

func TestExplainSymbolWithSourceHydratesFullContent(t *testing.T) {
	tools := seedSourceContentTools(t)
	tools.sourceHydrator = testHydrator("package server\nfunc ReserveProduct() {}\n", "text/plain")
	out, err := tools.ExplainSymbolWithSource(ExplainSymbolArgs{
		Repo:          "org/repo",
		Symbol:        "ReserveProduct",
		SourceContent: &SourceContentOptions{Mode: sourceContentModeFull},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.(SourceSymbolExplanation)
	if !ok {
		t.Fatalf("hydrated explain_symbol shape = %T, want SourceSymbolExplanation", out)
	}
	if got.Explanation.Node.NodeID != "n1" || !strings.Contains(got.SourceContent.Content, "ReserveProduct") {
		t.Fatalf("hydrated explain_symbol mismatch: %+v", got)
	}
}

func TestGetFileContextWithSourcePreservesDefaultShape(t *testing.T) {
	tools := seedSourceContentTools(t)
	out, err := tools.GetFileContextWithSource(GetFileContextArgs{Repo: "org/repo", File: "internal/server.go"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.([]store.NodeRow); !ok {
		t.Fatalf("default get_file_context shape = %T, want []store.NodeRow", out)
	}
}

func TestGetFileContextWithSourceHydratesFile(t *testing.T) {
	tools := seedSourceContentTools(t)
	tools.sourceHydrator = testHydrator("package server\nfunc ReserveProduct() {}\n", "text/plain")
	out, err := tools.GetFileContextWithSource(GetFileContextArgs{
		Repo:          "org/repo",
		File:          "internal/server.go",
		SourceContent: &SourceContentOptions{},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.(SourceFileContextResult)
	if !ok {
		t.Fatalf("hydrated get_file_context shape = %T, want SourceFileContextResult", out)
	}
	if got.File != "internal/server.go" || len(got.Symbols) != 1 || got.SourceContent.Status != sourceContentStatusFetched {
		t.Fatalf("hydrated get_file_context mismatch: %+v", got)
	}
}
