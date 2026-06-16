package godep

import "testing"

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
