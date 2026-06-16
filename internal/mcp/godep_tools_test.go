package mcp

import (
	"testing"

	"github.com/vend-ai/intel-mcp/internal/store"
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
