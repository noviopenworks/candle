package godep

import "testing"

func TestExtractUsages(t *testing.T) {
	deps := []Dependency{
		{ModulePath: "git.acme.local/platform/auth", Version: "v1.2.0", IsPrivate: true, Direct: true},
		{ModulePath: "github.com/spf13/viper", Version: "v1.21.0", IsPrivate: false, Direct: true},
	}
	usages, warns := extractUsages("testdata/consumer", deps)
	if len(warns) != 0 {
		t.Fatalf("warns: %v", warns)
	}
	var newClient *Usage
	for i := range usages {
		if usages[i].Symbol == "NewClient" {
			newClient = &usages[i]
		}
	}
	if newClient == nil {
		t.Fatalf("NewClient usage not found: %+v", usages)
	}
	if newClient.ModulePath != "git.acme.local/platform/auth" || newClient.Version != "v1.2.0" {
		t.Fatalf("usage module/version: %+v", newClient)
	}
	if newClient.PackagePath != "git.acme.local/platform/auth" || newClient.Line == 0 {
		t.Fatalf("usage package/line: %+v", newClient)
	}
	for _, u := range usages {
		if u.ModulePath == "github.com/spf13/viper" {
			t.Fatalf("public dep should not produce usages: %+v", u)
		}
	}
}
