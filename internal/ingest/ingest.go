package ingest

import (
	"fmt"
	"os"

	"github.com/vend-ai/intel-mcp/internal/config"
	"github.com/vend-ai/intel-mcp/internal/graph"
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
	}
	return rep, nil
}
