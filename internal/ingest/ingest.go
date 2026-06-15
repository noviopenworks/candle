package ingest

import (
	"fmt"
	"os"

	"github.com/vend-ai/intel-mcp/internal/config"
	"github.com/vend-ai/intel-mcp/internal/graph"
	"github.com/vend-ai/intel-mcp/internal/openapi"
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
