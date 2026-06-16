package godep

import "testing"

func TestExtractExports(t *testing.T) {
	lib, warns := extractExports("testdata/provider", "git.acme.local/platform/auth")
	if lib == nil {
		t.Fatalf("nil lib (warns=%v)", warns)
	}
	if !contains(lib.Readme, "Authentication helpers") {
		t.Fatalf("readme: %q", lib.Readme)
	}
	if !contains(lib.DocSynopsis, "token helpers") {
		t.Fatalf("doc synopsis: %q", lib.DocSynopsis)
	}
	got := map[string]string{} // symbol -> kind
	for _, e := range lib.Exports {
		got[e.Symbol] = e.Kind
	}
	if got["NewClient"] != "constructor" {
		t.Fatalf("NewClient kind: %q", got["NewClient"])
	}
	if got["Client"] != "type" {
		t.Fatalf("Client kind: %q", got["Client"])
	}
	if got["MaxRetries"] != "const" {
		t.Fatalf("MaxRetries kind: %q", got["MaxRetries"])
	}
	if _, ok := got["internalHelper"]; ok {
		t.Fatalf("unexported symbol leaked")
	}
	if _, ok := got["Verify"]; ok {
		t.Fatalf("method should not be a top-level export")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
