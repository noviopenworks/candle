package ingest

import (
	"fmt"
	"os"

	"github.com/vend-ai/intel-mcp/internal/config"
	"github.com/vend-ai/intel-mcp/internal/graph"
	"github.com/vend-ai/intel-mcp/internal/link"
	"github.com/vend-ai/intel-mcp/internal/openapi"
	"github.com/vend-ai/intel-mcp/internal/proto"
	"github.com/vend-ai/intel-mcp/internal/store"
)

// Report summarizes an ingestion run.
type Report struct {
	Indexed  int
	Skipped  int
	Warnings []string
}

// Run ingests every repo in cfg into the store. Missing graph files are
// skipped with a warning rather than aborting the whole run.
func Run(s *store.Store, cfg *config.Config) (Report, error) {
	var rep Report
	for _, r := range cfg.Repos {
		f, err := os.Open(r.Graph)
		if err != nil {
			rep.Skipped++
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: %v", r.Repo, err))
			continue
		}
		g, err := graph.Parse(f)
		f.Close()
		if err != nil {
			rep.Skipped++
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: parse: %v", r.Repo, err))
			continue
		}
		indexID, err := s.UpsertIndex(r.Org(), r.Name(), r.Commit, r.Branch, r.Graph)
		if err != nil {
			return rep, err
		}
		if _, err := graph.Load(s, indexID, g); err != nil {
			return rep, err
		}
		rep.Indexed++

		// OpenAPI specs (pure contract serving).
		var bundles []store.APISpecBundle
		for _, sp := range r.OpenAPI {
			spec, perr := openapi.ParseFile(sp)
			if perr != nil {
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: openapi %s: %v", r.Repo, sp, perr))
				continue
			}
			bundles = append(bundles, toBundle(spec, sp))
		}
		if err := s.ReplaceAPISpecs(indexID, bundles); err != nil {
			return rep, err
		}

		// Protobuf contracts.
		pfiles, pwarns, perr := proto.ParseFiles(r.Proto.Roots, r.Proto.Files)
		if perr != nil {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: proto: %v", r.Repo, perr))
		}
		for _, w := range pwarns {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: proto %s", r.Repo, w))
		}
		if err := s.ReplaceProtoFiles(indexID, toProtoBundles(pfiles)); err != nil {
			return rep, err
		}
		links, err := link.MatchRPCs(s, indexID, collectRPCs(pfiles))
		if err != nil {
			return rep, err
		}
		if err := s.LinkRPCImpls(indexID, links); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

func toBundle(spec *openapi.Spec, specPath string) store.APISpecBundle {
	b := store.APISpecBundle{Spec: store.APISpec{Kind: "openapi", Name: spec.Name, Version: spec.Version, Path: specPath}}
	for _, op := range spec.Operations {
		b.Operations = append(b.Operations, store.HTTPOperation{
			Method: op.Method, Path: op.Path, OperationID: op.OperationID, Summary: op.Summary,
			RequestSchema: op.RequestSchema, ResponseSchema: op.ResponseSchema, Security: op.Security, Tags: op.Tags,
		})
	}
	for _, sc := range spec.Schemas {
		b.Schemas = append(b.Schemas, store.APISchema{Name: sc.Name, Kind: "openapi_schema", RawRef: sc.RawRef})
	}
	return b
}

func toProtoBundles(files []proto.File) []store.ProtoFileBundle {
	var out []store.ProtoFileBundle
	for _, f := range files {
		b := store.ProtoFileBundle{File: store.ProtoFile{
			Path: f.Path, Package: f.Package, GoPackage: f.GoPackage, Imports: f.Imports}}
		for _, sv := range f.Services {
			sb := store.ProtoServiceBundle{Service: store.ProtoService{Name: sv.Name, FullName: sv.FullName}}
			for _, r := range sv.RPCs {
				sb.RPCs = append(sb.RPCs, store.ProtoRPC{
					Name: r.Name, FullName: r.FullName, RequestMessage: r.RequestMessage,
					ResponseMessage: r.ResponseMessage, StreamKind: r.StreamKind})
			}
			b.Services = append(b.Services, sb)
		}
		for _, m := range f.Messages {
			pm := store.ProtoMessage{Name: m.Name, FullName: m.FullName}
			for _, fld := range m.Fields {
				pm.Fields = append(pm.Fields, store.ProtoField{
					Name: fld.Name, Type: fld.Type, Number: fld.Number, Label: fld.Label})
			}
			b.Messages = append(b.Messages, pm)
		}
		for _, e := range f.Enums {
			pe := store.ProtoEnum{Name: e.Name, FullName: e.FullName}
			for _, v := range e.Values {
				pe.Values = append(pe.Values, store.ProtoEnumValue{Name: v.Name, Number: v.Number})
			}
			b.Enums = append(b.Enums, pe)
		}
		out = append(out, b)
	}
	return out
}

func collectRPCs(files []proto.File) []link.RPC {
	var out []link.RPC
	for _, f := range files {
		for _, sv := range f.Services {
			for _, r := range sv.RPCs {
				out = append(out, link.RPC{
					FullName: r.FullName, Service: sv.Name, Name: r.Name, StreamKind: r.StreamKind})
			}
		}
	}
	return out
}
