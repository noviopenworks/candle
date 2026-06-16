package store

import "testing"

func seedProto(t *testing.T) (*Store, int64) {
	t.Helper()
	s, _ := Open(":memory:")
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	bundle := ProtoFileBundle{
		File: ProtoFile{Path: "proto/inventory.proto", Package: "acme.inventory",
			GoPackage: "github.com/acme/inventory/gen", Imports: []string{"google/protobuf/timestamp.proto"}},
		Services: []ProtoServiceBundle{{
			Service: ProtoService{Name: "InventoryService", FullName: "acme.inventory.InventoryService"},
			RPCs: []ProtoRPC{{
				Name: "ReserveProduct", FullName: "acme.inventory.InventoryService.ReserveProduct",
				RequestMessage:  "acme.inventory.ReserveProductRequest",
				ResponseMessage: "acme.inventory.ReserveProductResponse", StreamKind: "unary"}},
		}},
		Messages: []ProtoMessage{{Name: "ReserveProductRequest", FullName: "acme.inventory.ReserveProductRequest",
			Fields: []ProtoField{{Name: "sku", Type: "string", Number: 1, Label: "optional"}}}},
		Enums: []ProtoEnum{{Name: "Status", FullName: "acme.inventory.Status",
			Values: []ProtoEnumValue{{Name: "OK", Number: 0}}}},
	}
	if err := s.ReplaceProtoFiles(id, []ProtoFileBundle{bundle}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	return s, id
}

func TestProtoStorageAndIdempotent(t *testing.T) {
	s, id := seedProto(t)
	defer s.Close()

	files, err := s.ListProtoFiles(id)
	if err != nil || len(files) != 1 || files[0].Package != "acme.inventory" {
		t.Fatalf("list files: %+v err=%v", files, err)
	}
	rpcs, err := s.FindRPCs(id, "reserve", "")
	if err != nil || len(rpcs) != 1 || rpcs[0].StreamKind != "unary" || rpcs[0].ProtoPath != "proto/inventory.proto" {
		t.Fatalf("find rpcs: %+v err=%v", rpcs, err)
	}
	if got, err := s.FindRPCs(id, "reserve", "bidi"); err != nil || len(got) != 0 {
		t.Fatalf("stream filter: %+v err=%v", got, err)
	}
	msgs, err := s.FindMessages(id, "Reserve")
	if err != nil || len(msgs) != 1 || len(msgs[0].Fields) != 1 {
		t.Fatalf("find messages: %+v err=%v", msgs, err)
	}

	if err := s.ReplaceProtoFiles(id, []ProtoFileBundle{}); err != nil {
		t.Fatalf("re-replace: %v", err)
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM proto_files WHERE index_id=?`, id).Scan(&n)
	if n != 0 {
		t.Fatalf("expected 0 files after empty replace, got %d", n)
	}
}

func TestLinkRPCImplsRoundTrip(t *testing.T) {
	s, id := seedProto(t)
	defer s.Close()

	rpcName := "acme.inventory.InventoryService.ReserveProduct"
	if err := s.LinkRPCImpls(id, []RPCImplLink{{
		RPCFullName: rpcName, NodeID: "n1", Confidence: 0.9, MatchReason: "name+service"}}); err != nil {
		t.Fatalf("link: %v", err)
	}
	impls, err := s.ProtoRPCImpls(id, rpcName)
	if err != nil || len(impls) != 1 || impls[0].NodeID != "n1" {
		t.Fatalf("impls: %+v err=%v", impls, err)
	}

	// Re-running replaces (idempotent, still 1).
	if err := s.LinkRPCImpls(id, []RPCImplLink{{
		RPCFullName: rpcName, NodeID: "n1", Confidence: 0.9, MatchReason: "name+service"}}); err != nil {
		t.Fatalf("relink: %v", err)
	}
	impls, err = s.ProtoRPCImpls(id, rpcName)
	if err != nil || len(impls) != 1 || impls[0].NodeID != "n1" {
		t.Fatalf("impls after relink: %+v err=%v", impls, err)
	}
}
