package mcp

import (
	"testing"

	"github.com/noviopenworks/candle/internal/store"
)

func seedExplain(t *testing.T) *Tools {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// Provider repo defines github.com/org/auth with one export node.
	pid, err := s.UpsertIndex("org", "auth-lib", "p1", "main", "/g/auth.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location) VALUES(?,?,?,?,?,?)`,
		pid, "n_validate", "ValidateToken", "code", "auth/token.go", "L5"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(pid, store.GoDepBundle{
		Libraries: []store.PrivateLibraryBundle{{
			Library: store.PrivateLibrary{ModulePath: "github.com/org/auth", DocSynopsis: "auth helpers"},
			Exports: []store.PrivateExport{{PackagePath: "github.com/org/auth", Symbol: "ValidateToken", Kind: "func", Doc: "validates", NodeID: "n_validate"}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	// Consumer repo with a node enclosing the usage at line 12 (def at L8).
	cid, err := s.UpsertIndex("org", "web", "c1", "main", "/g/web.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location) VALUES(?,?,?,?,?,?)`,
		cid, "n_login", "Login", "code", "internal/http/auth.go", "L8"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(cid, store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "github.com/org/auth", Version: "v1.2.0", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Usages:       []store.PrivateUsage{{ModulePath: "github.com/org/auth", Version: "v1.2.0", PackagePath: "github.com/org/auth", Symbol: "ValidateToken", File: "internal/http/auth.go", Line: 12}},
	}); err != nil {
		t.Fatal(err)
	}
	return NewTools(s)
}

func TestExplainPrivateLibraryProviderAndConsumers(t *testing.T) {
	tools := seedExplain(t)
	out, err := tools.ExplainPrivateLibrary("auth")
	if err != nil {
		t.Fatal(err)
	}
	if out.Provider.ModulePath != "github.com/org/auth" {
		t.Fatalf("provider module mismatch: %+v", out.Provider)
	}
	if out.Provider.Repo != "org/auth-lib" || out.Provider.Commit != "p1" {
		t.Fatalf("provider defining repo/commit mismatch: %+v", out.Provider)
	}
	if len(out.Provider.Exports) != 1 || !out.Provider.Exports[0].Resolved || out.Provider.Exports[0].Node == nil {
		t.Fatalf("expected resolved export node, got %+v", out.Provider.Exports)
	}
	if len(out.Consumers) != 1 || out.Consumers[0].Repo != "org/web" || out.Consumers[0].Version != "v1.2.0" {
		t.Fatalf("consumer mismatch: %+v", out.Consumers)
	}
	if len(out.Consumers[0].Usages) != 1 || !out.Consumers[0].Usages[0].Resolved || out.Consumers[0].Usages[0].Node == nil {
		t.Fatalf("expected resolved consumer usage link: %+v", out.Consumers[0].Usages)
	}
	if out.Consumers[0].Usages[0].Node.NodeID != "n_login" {
		t.Fatalf("expected enclosing node n_login, got %+v", out.Consumers[0].Usages[0].Node)
	}
	if len(out.Limitations) == 0 {
		t.Fatalf("expected limitations")
	}
}

func TestExplainPrivateLibraryUnknownQuery(t *testing.T) {
	tools := seedExplain(t)
	_, err := tools.ExplainPrivateLibrary("does-not-exist")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestExplainPrivateLibraryRespectsScope(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// Provider is in scope; consumer is indexed but outside the served scope.
	pid, err := s.UpsertIndex("org", "auth-lib", "p1", "main", "/g/a.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(pid, store.GoDepBundle{Libraries: []store.PrivateLibraryBundle{{
		Library: store.PrivateLibrary{ModulePath: "github.com/org/auth", DocSynopsis: "x"},
		Exports: []store.PrivateExport{{PackagePath: "github.com/org/auth", Symbol: "F"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	cid, err := s.UpsertIndex("org", "web", "c1", "main", "/g/web.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(cid, store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "github.com/org/auth", Version: "v1", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Usages:       []store.PrivateUsage{{ModulePath: "github.com/org/auth", Version: "v1", PackagePath: "github.com/org/auth", Symbol: "F", File: "x.go", Line: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	tools := NewToolsScoped(s, map[int64]bool{pid: true})
	out, err := tools.ExplainPrivateLibrary("github.com/org/auth")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Consumers) != 0 {
		t.Fatalf("out-of-scope consumer must be filtered, got %+v", out.Consumers)
	}
}

func TestExplainPrivateLibraryIgnoresOutOfScopeProvider(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	inScopeID, err := s.UpsertIndex("org", "auth-lib", "p1", "main", "/g/auth.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(inScopeID, store.GoDepBundle{Libraries: []store.PrivateLibraryBundle{{
		Library: store.PrivateLibrary{ModulePath: "github.com/org/auth"},
		Exports: []store.PrivateExport{{PackagePath: "github.com/org/auth", Symbol: "F"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	outOfScopeID, err := s.UpsertIndex("org", "platform-go", "p2", "main", "/g/platform.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(outOfScopeID, store.GoDepBundle{Libraries: []store.PrivateLibraryBundle{{
		Library: store.PrivateLibrary{ModulePath: "github.com/org/platform-go"},
		Exports: []store.PrivateExport{{PackagePath: "github.com/org/platform-go", Symbol: "G"}},
	}}}); err != nil {
		t.Fatal(err)
	}

	tools := NewToolsScoped(s, map[int64]bool{inScopeID: true})
	_, err = tools.ExplainPrivateLibrary("platform")
	if err != ErrNotFound {
		t.Fatalf("out-of-scope provider should be hidden, got %v", err)
	}
}

func TestExplainPrivateLibraryProviderLess(t *testing.T) {
	// A module consumed but with no indexed provider: provider section empty,
	// consumers still returned, no error.
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	cid, err := s.UpsertIndex("org", "web", "c1", "main", "/g/web.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(cid, store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "github.com/org/nostub", Version: "v0.1.0", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Usages:       []store.PrivateUsage{{ModulePath: "github.com/org/nostub", Version: "v0.1.0", PackagePath: "github.com/org/nostub", Symbol: "Do", File: "x.go", Line: 3}},
	}); err != nil {
		t.Fatal(err)
	}
	tools := NewTools(s)
	out, err := tools.ExplainPrivateLibrary("nostub")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Provider.Exports) != 0 {
		t.Fatalf("expected no provider exports, got %+v", out.Provider.Exports)
	}
	if len(out.Consumers) != 1 || out.Consumers[0].Repo != "org/web" {
		t.Fatalf("expected consumer org/web, got %+v", out.Consumers)
	}
	// No node in x.go -> usage link unresolved, not errored.
	if out.Consumers[0].Usages[0].Resolved {
		t.Fatalf("expected unresolved consumer link, got %+v", out.Consumers[0].Usages[0])
	}
}

func TestExplainPrivateLibraryAmbiguousReturnsCandidates(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.UpsertIndex("org", "web", "c1", "main", "/g/web.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(id, store.GoDepBundle{
		Dependencies: []store.Dependency{
			{ModulePath: "github.com/org/authcore", Version: "v1.0.0", Ecosystem: "go", IsPrivate: true, Direct: true},
			{ModulePath: "github.com/org/authutil", Version: "v1.0.0", Ecosystem: "go", IsPrivate: true, Direct: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	tools := NewTools(s)
	out, err := tools.ExplainPrivateLibrary("auth")
	if err != nil {
		t.Fatal(err)
	}
	if out.Provider.ModulePath == "" {
		t.Fatalf("expected a best match")
	}
	if len(out.Candidates) != 1 {
		t.Fatalf("expected 1 candidate alongside best match, got %+v", out.Candidates)
	}
}
