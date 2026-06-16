package store

import "testing"

func seedGoDeps(t *testing.T) (*Store, int64) {
	t.Helper()
	s, _ := Open(":memory:")
	id, _ := s.UpsertIndex("acme", "web", "abc", "main", "/g")
	bundle := GoDepBundle{
		Dependencies: []Dependency{
			{ModulePath: "git.acme.local/platform/auth", Version: "v1.2.0", Ecosystem: "go", IsPrivate: true, Direct: true},
			{ModulePath: "github.com/spf13/viper", Version: "v1.21.0", Ecosystem: "go", IsPrivate: false, Direct: true},
		},
		Libraries: []PrivateLibraryBundle{{
			Library: PrivateLibrary{ModulePath: "git.acme.local/platform/auth", Readme: "Auth helpers", DocSynopsis: "Package auth provides tokens"},
			Exports: []PrivateExport{{PackagePath: "git.acme.local/platform/auth", Symbol: "NewClient", Kind: "constructor", Doc: "NewClient builds a client", NodeID: "n1"}},
		}},
		Usages: []PrivateUsage{
			{ModulePath: "git.acme.local/platform/auth", Version: "v1.2.0", PackagePath: "git.acme.local/platform/auth", Symbol: "NewClient", File: "main.go", Line: 12},
		},
	}
	if err := s.ReplaceGoDeps(id, bundle); err != nil {
		t.Fatalf("replace: %v", err)
	}
	return s, id
}

func TestGoDepStorageAndIdempotent(t *testing.T) {
	s, id := seedGoDeps(t)
	defer s.Close()

	libs, err := s.FindPrivateLibraries(id, "auth")
	if err != nil || len(libs) != 1 || libs[0].ModulePath != "git.acme.local/platform/auth" || libs[0].ExportCount != 1 {
		t.Fatalf("find libs: %+v err=%v", libs, err)
	}
	dep, ok, err := s.DependencyByModule(id, "git.acme.local/platform/auth")
	if err != nil || !ok || dep.Version != "v1.2.0" {
		t.Fatalf("dep: %+v ok=%v err=%v", dep, ok, err)
	}
	usages, err := s.PrivateUsagesByModule(id, "git.acme.local/platform/auth")
	if err != nil || len(usages) != 1 || usages[0].Symbol != "NewClient" || usages[0].Line != 12 {
		t.Fatalf("usages: %+v err=%v", usages, err)
	}
	lib, ok, err := s.PrivateLibraryByModule("git.acme.local/platform/auth")
	if err != nil || !ok || len(lib.Exports) != 1 || lib.Exports[0].NodeID != "n1" {
		t.Fatalf("lib by module: %+v ok=%v err=%v", lib, ok, err)
	}

	// Idempotent: empty replace clears.
	if err := s.ReplaceGoDeps(id, GoDepBundle{}); err != nil {
		t.Fatalf("re-replace: %v", err)
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM dependencies WHERE index_id=?`, id).Scan(&n)
	if n != 0 {
		t.Fatalf("expected 0 deps after empty replace, got %d", n)
	}
}
