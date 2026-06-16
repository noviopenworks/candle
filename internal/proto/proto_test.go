package proto

import "testing"

func TestParseInventory(t *testing.T) {
	files, warns, err := ParseFiles([]string{"testdata"}, []string{"inventory.proto"})
	if err != nil {
		t.Fatalf("parse: %v (warns=%v)", err, warns)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Package != "acme.inventory" || f.GoPackage != "github.com/acme/inventory/gen" {
		t.Fatalf("file meta: %+v", f)
	}
	if len(f.Services) != 1 || len(f.Services[0].RPCs) != 4 {
		t.Fatalf("services: %+v", f.Services)
	}
	kinds := map[string]string{}
	for _, r := range f.Services[0].RPCs {
		kinds[r.Name] = r.StreamKind
	}
	want := map[string]string{
		"ReserveProduct": "unary", "WatchStock": "server_stream",
		"UploadCounts": "client_stream", "Sync": "bidi",
	}
	for name, sk := range want {
		if kinds[name] != sk {
			t.Fatalf("rpc %s stream_kind=%q want %q", name, kinds[name], sk)
		}
	}
	var reserve *RPC
	for i := range f.Services[0].RPCs {
		if f.Services[0].RPCs[i].Name == "ReserveProduct" {
			reserve = &f.Services[0].RPCs[i]
		}
	}
	if reserve.RequestMessage != "acme.inventory.ReserveProductRequest" {
		t.Fatalf("request msg: %q", reserve.RequestMessage)
	}
	if reserve.FullName != "acme.inventory.InventoryService.ReserveProduct" {
		t.Fatalf("rpc full name: %q", reserve.FullName)
	}
	var req *Message
	for i := range f.Messages {
		if f.Messages[i].Name == "ReserveProductRequest" {
			req = &f.Messages[i]
		}
	}
	if req == nil || len(req.Fields) != 2 || req.Fields[0].Name != "sku" {
		t.Fatalf("message fields: %+v", req)
	}
	var status *Enum
	for i := range f.Enums {
		if f.Enums[i].FullName == "acme.inventory.Status" {
			status = &f.Enums[i]
		}
	}
	if status == nil || status.Name != "Status" || len(status.Values) != 2 {
		t.Fatalf("enums: %+v", f.Enums)
	}
}

func TestParseNestedTypes(t *testing.T) {
	files, warns, err := ParseFiles([]string{"testdata"}, []string{"inventory.proto"})
	if err != nil {
		t.Fatalf("parse: %v (warns=%v)", err, warns)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 file, got %d", len(files))
	}
	f := files[0]
	foundMsg := false
	for _, m := range f.Messages {
		if m.FullName == "acme.inventory.Warehouse.Bin" {
			foundMsg = true
		}
	}
	if !foundMsg {
		t.Fatalf("nested message acme.inventory.Warehouse.Bin not found; messages=%+v", f.Messages)
	}
	foundEnum := false
	for _, e := range f.Enums {
		if e.FullName == "acme.inventory.Warehouse.Kind" {
			foundEnum = true
		}
	}
	if !foundEnum {
		t.Fatalf("nested enum acme.inventory.Warehouse.Kind not found; enums=%+v", f.Enums)
	}
}

func TestParseDirectoryEntry(t *testing.T) {
	// "sub" is a DIRECTORY entry relative to the root "testdata"; it must expand
	// to the .proto files beneath it.
	files, warns, err := ParseFiles([]string{"testdata"}, []string{"sub"})
	if err != nil {
		t.Fatalf("parse: %v (warns=%v)", err, warns)
	}
	var extra *File
	for i := range files {
		if files[i].Package == "acme.extra" {
			extra = &files[i]
		}
	}
	if extra == nil {
		t.Fatalf("acme.extra file not found; files=%+v warns=%v", files, warns)
	}
	if len(extra.Services) != 1 || extra.Services[0].Name != "ExtraService" {
		t.Fatalf("services: %+v", extra.Services)
	}
	if len(extra.Services[0].RPCs) != 1 || extra.Services[0].RPCs[0].Name != "Ping" {
		t.Fatalf("rpcs: %+v", extra.Services[0].RPCs)
	}
}

func TestParseMissingFileWarns(t *testing.T) {
	files, warns, err := ParseFiles([]string{"testdata"}, []string{"nope.proto"})
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if len(files) != 0 || len(warns) == 0 {
		t.Fatalf("want 0 files + a warning, got files=%d warns=%v", len(files), warns)
	}
}
