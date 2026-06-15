package config

import "testing"

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

func TestInvalidRepoIdentity(t *testing.T) {
	_, err := (RepoConfig{Repo: "noslash"}).validate()
	if err == nil {
		t.Fatal("expected error for repo without org/name slash")
	}
}
