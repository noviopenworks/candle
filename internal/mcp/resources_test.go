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

func TestOperationResource(t *testing.T) {
	tl := seedAPITools(t)
	body, err := tl.OperationResource("org/svc", "reserveProduct")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "reserveProduct") || !strings.Contains(body, "ReservationResponse") {
		t.Fatalf("expected operation contract, got %q", body)
	}
	if _, err := tl.OperationResource("org/svc", "nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSchemaResource(t *testing.T) {
	tl := seedAPITools(t)
	body, err := tl.SchemaResource("org/svc", "ReservationResponse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "ReservationResponse") {
		t.Fatalf("expected schema, got %q", body)
	}
	// Substring (non-exact) must not match — resource requires exact name.
	if _, err := tl.SchemaResource("org/svc", "Reservation"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for non-exact name, got %v", err)
	}
}

func TestSpecResource(t *testing.T) {
	tl := seedAPITools(t)
	body, err := tl.SpecResource("org/svc", "api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Inventory API") || !strings.Contains(body, "reserveProduct") {
		t.Fatalf("expected spec + its operations, got %q", body)
	}
	if _, err := tl.SpecResource("org/svc", "missing.yaml"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestParseOpenAPIURI(t *testing.T) {
	repo, kind, ref, err := parseOpenAPIURI("openapi://org/svc/commit/abc/spec/api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if repo != "org/svc" || kind != "spec" || ref != "api/openapi.yaml" {
		t.Fatalf("bad parse: repo=%q kind=%q ref=%q", repo, kind, ref)
	}
	if _, _, _, err := parseOpenAPIURI("openapi://org/svc/bad"); err == nil {
		t.Fatalf("expected error for malformed uri")
	}
}
