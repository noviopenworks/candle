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

func TestInvalidRepoIdentity(t *testing.T) {
	_, err := (RepoConfig{Repo: "noslash"}).validate()
	if err == nil {
		t.Fatal("expected error for repo without org/name slash")
	}
}
