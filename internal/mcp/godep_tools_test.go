package mcp

import (
	"testing"

	"github.com/noviopenworks/candlegraph/internal/store"
)

func seedGoDepTools(t *testing.T) *Tools {
	t.Helper()
	s, _ := store.Open(":memory:")
	id, _ := s.UpsertIndex("acme", "web", "abc", "main", "/g")
	bundle := store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "git.acme.local/platform/auth", Version: "v1.2.0", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Libraries: []store.PrivateLibraryBundle{{
			Library: store.PrivateLibrary{ModulePath: "git.acme.local/platform/auth", DocSynopsis: "Package auth provides tokens"},
			Exports: []store.PrivateExport{{PackagePath: "git.acme.local/platform/auth", Symbol: "NewClient", Kind: "constructor"}},
		}},
		Usages: []store.PrivateUsage{{ModulePath: "git.acme.local/platform/auth", Version: "v1.2.0", PackagePath: "git.acme.local/platform/auth", Symbol: "NewClient", File: "main.go", Line: 12}},
	}
	if err := s.ReplaceGoDeps(id, bundle); err != nil {
		t.Fatal(err)
	}
	return NewTools(s)
}

func TestFindPrivateLibrary(t *testing.T) {
	tools := seedGoDepTools(t)
	got, err := tools.FindPrivateLibrary("acme/web", "auth")
	if err != nil || len(got) != 1 || got[0].ModulePath != "git.acme.local/platform/auth" || got[0].ExportCount != 1 {
		t.Fatalf("find: %+v err=%v", got, err)
	}
}

func TestFindPrivateLibraryPathOnly(t *testing.T) {
	s, _ := store.Open(":memory:")
	id, _ := s.UpsertIndex("acme", "web", "abc", "main", "/g")
	bundle := store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "git.acme.local/platform/other", Version: "v0.3.0", Ecosystem: "go", IsPrivate: true, Direct: true}},
	}
	if err := s.ReplaceGoDeps(id, bundle); err != nil {
		t.Fatal(err)
	}
	tools := NewTools(s)
	got, err := tools.FindPrivateLibrary("acme/web", "other")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 result, got %d: %+v", len(got), got)
	}
	if got[0].ModulePath != "git.acme.local/platform/other" || got[0].ExportCount != 0 {
		t.Fatalf("path-only fallback result: %+v", got[0])
	}
}

func TestFindLibraryConsumers(t *testing.T) {
	tools := seedGoDepTools(t)
	out, err := tools.FindLibraryConsumers("acme/web", "git.acme.local/platform/auth")
	if err != nil {
		t.Fatalf("consumers: %v", err)
	}
	if out.Version != "v1.2.0" || len(out.UsedSymbols) != 1 || out.UsedSymbols[0].Symbol != "NewClient" {
		t.Fatalf("shape: %+v", out)
	}
	if out.ConsumedAcrossRepos == "" {
		t.Fatalf("expected deferred marker")
	}
	if _, err := tools.FindLibraryConsumers("acme/web", "git.acme.local/none"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
