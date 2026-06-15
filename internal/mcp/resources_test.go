package mcp

import (
	"strings"
	"testing"
)

func TestRepoResource(t *testing.T) {
	tl := seedTools(t)
	body, err := tl.RepoResource("org/svc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "org/svc") {
		t.Fatalf("expected repo identity in body, got %q", body)
	}
}

func TestGraphNodeResource(t *testing.T) {
	tl := seedTools(t)
	body, err := tl.GraphNodeResource("org/svc", "n1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "ReserveProduct") {
		t.Fatalf("expected node label, got %q", body)
	}
}

func TestGraphNodeResourceUnknown(t *testing.T) {
	tl := seedTools(t)
	if _, err := tl.GraphNodeResource("org/svc", "nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
