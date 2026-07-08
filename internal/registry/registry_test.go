package registry

import (
	"testing"

	"github.com/noviopenworks/candle/internal/store"
)

func TestResolveExactAndFuzzy(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	s.UpsertIndex("org", "inventory-service", "abc", "main", "/g")

	r := New(s)

	got, ok, err := r.Resolve("org/inventory-service")
	if err != nil || !ok {
		t.Fatalf("exact resolve failed: ok=%v err=%v", ok, err)
	}
	if got.Repo != "org/inventory-service" {
		t.Fatalf("unexpected repo %q", got.Repo)
	}

	// Fuzzy: partial name.
	m, err := r.Match("inventory")
	if err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 || m[0].Repo != "org/inventory-service" {
		t.Fatalf("fuzzy match failed: %+v", m)
	}
}
