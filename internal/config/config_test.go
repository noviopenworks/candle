package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	cfg, err := Load("testdata/manifest.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.Repos))
	}
	r := cfg.Repos[0]
	if r.Org() != "org" || r.Name() != "inventory-service" {
		t.Fatalf("bad split: org=%q name=%q", r.Org(), r.Name())
	}
	if r.Commit != "abc123" {
		t.Fatalf("expected commit abc123, got %q", r.Commit)
	}
}

func TestRepoRoot(t *testing.T) {
	cfg, err := Load("testdata/manifest.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Repos[0].Root; got != "/abs/inventory" {
		t.Fatalf("expected root /abs/inventory, got %q", got)
	}
	// A repo entry that omits root must load fine with an empty Root.
	if got := cfg.Repos[1].Root; got != "" {
		t.Fatalf("expected empty root for repo without root, got %q", got)
	}
}

func TestOpenAPIPaths(t *testing.T) {
	cfg, err := Load("testdata/manifest.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Repos[0].OpenAPI) != 1 || cfg.Repos[0].OpenAPI[0] != "api/openapi.yaml" {
		t.Fatalf("expected one openapi path, got %+v", cfg.Repos[0].OpenAPI)
	}
}

func TestProtoConfigParses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	yaml := "repos:\n" +
		"  - repo: acme/inventory\n" +
		"    graph: /tmp/g.json\n" +
		"    proto:\n" +
		"      roots: [proto]\n" +
		"      files: [proto/inventory.proto]\n"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	r := cfg.Repos[0]
	if len(r.Proto.Roots) != 1 || r.Proto.Roots[0] != "proto" {
		t.Fatalf("roots: %+v", r.Proto.Roots)
	}
	if len(r.Proto.Files) != 1 || r.Proto.Files[0] != "proto/inventory.proto" {
		t.Fatalf("files: %+v", r.Proto.Files)
	}
}

func TestGoConfigParses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	yaml := "repos:\n" +
		"  - repo: acme/web\n" +
		"    graph: /tmp/g.json\n" +
		"    go:\n" +
		"      modules: [go.mod]\n" +
		"      private_prefixes: [git.acme.local/]\n"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	r := cfg.Repos[0]
	if len(r.Go.Modules) != 1 || r.Go.Modules[0] != "go.mod" {
		t.Fatalf("modules: %+v", r.Go.Modules)
	}
	if len(r.Go.PrivatePrefixes) != 1 || r.Go.PrivatePrefixes[0] != "git.acme.local/" {
		t.Fatalf("prefixes: %+v", r.Go.PrivatePrefixes)
	}
}

func TestInvalidRepoIdentity(t *testing.T) {
	err := (RepoConfig{Repo: "noslash"}).validate()
	if err == nil {
		t.Fatal("expected error for repo without org/name slash")
	}
}
