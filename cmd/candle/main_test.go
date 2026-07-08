package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noviopenworks/candle/internal/store"
)

func TestRunServeResolvesScopeConfig(t *testing.T) {
	tests := []struct {
		name           string
		explicitConfig bool
		manifestArg    string
		writeDefault   string
		writeExplicit  string
		wantScoped     bool
		wantRepos      []string
		wantErr        bool
	}{
		{
			name:           "explicit config wins over cwd manifest",
			explicitConfig: true,
			manifestArg:    "explicit.yaml",
			writeDefault:   manifestFor("org/other", "other1"),
			writeExplicit:  manifestFor("org/web", "web1"),
			wantScoped:     true,
			wantRepos:      []string{"org/web"},
		},
		{
			name:         "discovers default manifest in cwd",
			manifestArg:  "candle.yaml",
			writeDefault: manifestFor("org/other", "other1"),
			wantScoped:   true,
			wantRepos:    []string{"org/other"},
		},
		{
			name:        "absent config serves all",
			manifestArg: "candle.yaml",
			wantScoped:  false,
		},
		{
			name:           "missing explicit config returns load error",
			explicitConfig: true,
			manifestArg:    "missing.yaml",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Chdir(tmp)

			dbPath := filepath.Join(tmp, "intel.db")
			seedServeStore(t, dbPath)

			if tt.writeDefault != "" {
				writeFile(t, filepath.Join(tmp, "candle.yaml"), tt.writeDefault)
			}
			manifestArg := tt.manifestArg
			if tt.writeExplicit != "" {
				manifestArg = filepath.Join(tmp, tt.manifestArg)
				writeFile(t, manifestArg, tt.writeExplicit)
			}

			var stderr bytes.Buffer
			serveCalled := false
			var scopedRepos []string
			err := runServe(context.Background(), dbPath, manifestArg, tt.explicitConfig, &stderr,
				func(context.Context, *store.Store) error {
					serveCalled = true
					return nil
				},
				func(_ context.Context, s *store.Store, allowed map[int64]bool) error {
					for _, repo := range []string{"org/web", "org/other"} {
						if scopeContainsRepo(t, s, allowed, repo) {
							scopedRepos = append(scopedRepos, repo)
						}
					}
					return nil
				})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if serveCalled || scopedRepos != nil {
					t.Fatalf("serve functions should not run on config load error; serveCalled=%v scopedRepos=%v", serveCalled, scopedRepos)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantScoped {
				if serveCalled {
					t.Fatal("unscoped serve should not run when scope config is present")
				}
				if strings.Join(scopedRepos, ",") != strings.Join(tt.wantRepos, ",") {
					t.Fatalf("scoped repos = %v, want %v", scopedRepos, tt.wantRepos)
				}
				if !strings.Contains(stderr.String(), "serving 1 configured snapshot(s)") {
					t.Fatalf("expected startup scope summary on stderr, got %q", stderr.String())
				}
				return
			}
			if !serveCalled {
				t.Fatal("unscoped serve should run when no config is present")
			}
			if scopedRepos != nil {
				t.Fatalf("scoped serve should not run without config, got %v", scopedRepos)
			}
		})
	}
}

func manifestFor(repo, commit string) string {
	return "repos:\n" +
		"  - repo: " + repo + "\n" +
		"    graph: /tmp/graph.json\n" +
		"    commit: " + commit + "\n"
}

func seedServeStore(t *testing.T, dbPath string) {
	t.Helper()
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.UpsertIndex("org", "web", "web1", "main", "/tmp/web.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertIndex("org", "other", "other1", "main", "/tmp/other.json"); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scopeContainsRepo(t *testing.T, s *store.Store, allowed map[int64]bool, repo string) bool {
	t.Helper()
	for id := range allowed {
		var got string
		if err := s.DB.QueryRow(`
			SELECT r.org || '/' || r.name
			FROM indexes i JOIN repos r ON r.id=i.repo_id
			WHERE i.id=?`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got == repo {
			return true
		}
	}
	return false
}
