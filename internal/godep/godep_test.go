package godep

import (
	"strings"
	"testing"
)

func TestParseConsumerModule(t *testing.T) {
	res, warns, err := Parse([]string{"testdata/consumer/go.mod"}, []string{"git.acme.local/"})
	if err != nil {
		t.Fatalf("parse: %v (warns=%v)", err, warns)
	}
	if res.ModulePath != "git.acme.local/apps/web" {
		t.Fatalf("module path: %q", res.ModulePath)
	}
	byPath := map[string]Dependency{}
	for _, d := range res.Dependencies {
		byPath[d.ModulePath] = d
	}
	auth, ok := byPath["git.acme.local/platform/auth"]
	if !ok || auth.Version != "v1.2.0" || !auth.IsPrivate || !auth.Direct {
		t.Fatalf("auth dep: %+v ok=%v", auth, ok)
	}
	viper, ok := byPath["github.com/spf13/viper"]
	if !ok || viper.IsPrivate || viper.Direct {
		t.Fatalf("viper dep should be public+indirect: %+v ok=%v", viper, ok)
	}
}

func TestParseReplaceDirective(t *testing.T) {
	res, warns, err := Parse([]string{"testdata/replace/go.mod"}, []string{"git.acme.local/"})
	if err != nil {
		t.Fatalf("parse: %v (warns=%v)", err, warns)
	}
	var auth *Dependency
	for i := range res.Dependencies {
		if res.Dependencies[i].ModulePath == "git.acme.local/platform/auth" {
			auth = &res.Dependencies[i]
		}
	}
	if auth == nil {
		t.Fatalf("auth dep not found: %+v", res.Dependencies)
	}
	if auth.Version != "" {
		t.Fatalf("local replacement should clear version, got %q", auth.Version)
	}
	if !auth.IsPrivate {
		t.Fatalf("auth should still be private after replace: %+v", auth)
	}
}

func TestParseGoWork(t *testing.T) {
	res, warns, err := Parse([]string{"testdata/workspace/go.work"}, []string{"git.acme.local/"})
	if err != nil {
		t.Fatalf("parse: %v (warns=%v)", err, warns)
	}
	var auth *Dependency
	for i := range res.Dependencies {
		if res.Dependencies[i].ModulePath == "git.acme.local/platform/auth" {
			auth = &res.Dependencies[i]
		}
	}
	if auth == nil {
		t.Fatalf("workspace did not resolve moda's auth dependency: %+v", res.Dependencies)
	}
	if auth.Version != "v0.9.0" || !auth.IsPrivate {
		t.Fatalf("workspace auth dep: %+v", auth)
	}
}

func TestParseGoSumMismatch(t *testing.T) {
	_, warns, err := Parse([]string{"testdata/summismatch/go.mod"}, []string{"git.acme.local/"})
	if err != nil {
		t.Fatalf("parse should succeed without hard error: %v", err)
	}
	found := false
	for _, w := range warns {
		if strings.Contains(w, "go.sum") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a go.sum mismatch warning, got: %v", warns)
	}
}
