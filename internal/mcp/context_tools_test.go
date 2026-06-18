package mcp

import (
	"testing"

	"github.com/noviopenworks/candlegraph/internal/store"
)

func seedContextTools(t *testing.T) *Tools {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.UpsertIndex("org", "inventory", "abc123", "main", "/graphs/inventory/graph.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location) VALUES(?,?,?,?,?,?)`,
		id, "handler_reserve", "ReserveProduct", "code", "internal/http/reservation_handler.go", "L10"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location) VALUES(?,?,?,?,?,?)`,
		id, "service_reserve", "ReserveService", "code", "internal/reservation/service.go", "L20"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`INSERT INTO edges(index_id,source,target,relation) VALUES(?,?,?,?)`, id, "handler_reserve", "service_reserve", "calls"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceAPISpecs(id, []store.APISpecBundle{{
		Spec:       store.APISpec{Kind: "openapi", Name: "Inventory API", Version: "1.0.0", Path: "api/openapi.yaml"},
		Operations: []store.HTTPOperation{{Method: "POST", Path: "/v1/reservations", OperationID: "reserveProduct", Summary: "Reserve product", RequestSchema: "ReserveProductRequest", ResponseSchema: "Reservation"}},
		Schemas:    []store.APISchema{{Name: "ReserveProductRequest", Kind: "openapi_schema"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceProtoFiles(id, []store.ProtoFileBundle{{
		File: store.ProtoFile{Path: "proto/inventory.proto", Package: "inventory.v1"},
		Services: []store.ProtoServiceBundle{{
			Service: store.ProtoService{Name: "InventoryService", FullName: "inventory.v1.InventoryService"},
			RPCs:    []store.ProtoRPC{{Name: "ReserveProduct", FullName: "inventory.v1.InventoryService.ReserveProduct", RequestMessage: "inventory.v1.ReserveProductRequest", ResponseMessage: "inventory.v1.ReserveProductResponse", StreamKind: "unary"}},
		}},
		Messages: []store.ProtoMessage{{Name: "ReserveProductRequest", FullName: "inventory.v1.ReserveProductRequest"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(id, store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "github.com/org/auth", Version: "v1.2.3", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Usages:       []store.PrivateUsage{{ModulePath: "github.com/org/auth", Version: "v1.2.3", PackagePath: "github.com/org/auth", Symbol: "ValidateToken", File: "internal/http/auth.go", Line: 12}},
	}); err != nil {
		t.Fatal(err)
	}
	return NewTools(s)
}

func TestGetContextOverview(t *testing.T) {
	tools := seedContextTools(t)
	out, err := tools.GetContext(GetContextArgs{Repo: "org/inventory"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Repo.Repo != "org/inventory" || out.Repo.Commit != "abc123" {
		t.Fatalf("repo summary mismatch: %+v", out.Repo)
	}
	if out.Topic != "" {
		t.Fatalf("overview topic should be empty: %q", out.Topic)
	}
	if out.Capabilities.CodeGraph.Count != 2 || !out.Capabilities.CodeGraph.Available {
		t.Fatalf("code graph capability mismatch: %+v", out.Capabilities.CodeGraph)
	}
	if out.Capabilities.OpenAPI.Count != 1 || out.Capabilities.Protobuf.Count != 1 || out.Capabilities.PrivateLibraries.Count != 1 {
		t.Fatalf("capabilities mismatch: %+v", out.Capabilities)
	}
	if len(out.SuggestedNextCalls) == 0 {
		t.Fatalf("expected suggested next calls")
	}
	if len(out.ResourceSchemes) == 0 {
		t.Fatalf("expected resource schemes")
	}
	if len(out.Limitations) == 0 {
		t.Fatalf("expected limitations")
	}
}

func TestGetContextTopicSearchesAllSurfaces(t *testing.T) {
	tools := seedContextTools(t)
	out, err := tools.GetContext(GetContextArgs{Repo: "org/inventory", Topic: "ReserveProduct", IncludeResources: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.Topic != "ReserveProduct" {
		t.Fatalf("topic mismatch: %q", out.Topic)
	}
	if len(out.Matches.CodeSymbols) != 1 {
		t.Fatalf("expected one code symbol match, got %+v", out.Matches.CodeSymbols)
	}
	if len(out.Matches.CodeSymbols[0].Callees) != 1 {
		t.Fatalf("expected codegraph-like callees: %+v", out.Matches.CodeSymbols[0])
	}
	if len(out.Matches.Schemas) == 0 {
		t.Fatalf("expected schema match")
	}
	if len(out.Matches.RPCs) == 0 {
		t.Fatalf("expected rpc match")
	}
	if len(out.Resources) == 0 {
		t.Fatalf("expected resource URIs")
	}
}

func TestGetContextCodeModeOnlyReturnsCode(t *testing.T) {
	tools := seedContextTools(t)
	out, err := tools.GetContext(GetContextArgs{Repo: "org/inventory", Topic: "ReserveProduct", Mode: "code"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Matches.CodeSymbols) != 1 {
		t.Fatalf("expected code symbol match")
	}
	if len(out.Matches.Schemas) != 0 || len(out.Matches.RPCs) != 0 || out.Matches.Endpoints != nil {
		t.Fatalf("code mode should not include non-code matches: %+v", out.Matches)
	}
}

func TestGetContextOverviewModeSuppressesMatches(t *testing.T) {
	tools := seedContextTools(t)
	out, err := tools.GetContext(GetContextArgs{Repo: "org/inventory", Topic: "ReserveProduct", Mode: "overview"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Matches.CodeSymbols) != 0 || len(out.Matches.Schemas) != 0 || len(out.Matches.RPCs) != 0 || out.Matches.Endpoints != nil || out.Matches.PrivateLibraries != nil {
		t.Fatalf("overview mode must suppress matches: %+v", out.Matches)
	}
	if out.Capabilities.CodeGraph.Count != 2 {
		t.Fatalf("overview mode must still return the catalog")
	}
}

func TestGetContextUnknownRepo(t *testing.T) {
	tools := seedContextTools(t)
	_, err := tools.GetContext(GetContextArgs{Repo: "org/missing"})
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
