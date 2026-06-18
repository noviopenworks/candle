package mcp

import (
	"strconv"
	"strings"

	"github.com/noviopenworks/candlegraph/internal/store"
)

// LibraryExplanation is the explain_private_library result: provider definition
// plus cross-repo consumers, with code-graph links where resolvable.
type LibraryExplanation struct {
	Query       string         `json:"query"`
	Provider    ProviderInfo   `json:"provider"`
	Consumers   []ConsumerInfo `json:"consumers"`
	Candidates  []string       `json:"candidates,omitempty"`
	Limitations []string       `json:"limitations"`
}

// ProviderInfo is the provider side of a private library.
type ProviderInfo struct {
	ModulePath  string       `json:"module_path"`
	Repo        string       `json:"repo,omitempty"`
	Commit      string       `json:"commit,omitempty"`
	DocSynopsis string       `json:"doc_synopsis,omitempty"`
	Packages    []string     `json:"packages,omitempty"`
	Exports     []ExportInfo `json:"exports,omitempty"`
}

// ExportInfo is one provider export with an optional code-graph link.
type ExportInfo struct {
	PackagePath string         `json:"package_path"`
	Symbol      string         `json:"symbol"`
	Kind        string         `json:"kind,omitempty"`
	Doc         string         `json:"doc,omitempty"`
	Node        *store.NodeRow `json:"node,omitempty"`
	Resolved    bool           `json:"resolved"`
}

// ConsumerInfo is one consuming repo.
type ConsumerInfo struct {
	Repo         string      `json:"repo"`
	Commit       string      `json:"commit,omitempty"`
	Version      string      `json:"version,omitempty"`
	UsedPackages []string    `json:"used_packages,omitempty"`
	Usages       []UsageLink `json:"usages,omitempty"`
}

// UsageLink is one usage with an optional best-effort consumer node link.
type UsageLink struct {
	Usage    store.PrivateUsage `json:"usage"`
	Node     *store.NodeRow     `json:"node,omitempty"`
	Resolved bool               `json:"resolved"`
}

func explainLimitations() []string {
	return []string{
		"Version-diff and breaking-change analysis are out of scope for explain_private_library.",
		"Multi-hop call-path expansion and transitive dependents are deferred.",
		"Only Go private libraries are supported.",
	}
}

// ExplainPrivateLibrary implements explain_private_library: resolve a fuzzy
// query to a private library, then explain provider exports and cross-repo
// consumers with code-graph links.
func (t *Tools) ExplainPrivateLibrary(query string) (LibraryExplanation, error) {
	paths, err := t.s.SearchPrivateModulePaths(query)
	if err != nil {
		return LibraryExplanation{}, err
	}
	if len(paths) == 0 {
		return LibraryExplanation{}, ErrNotFound
	}
	best := paths[0]
	for _, p := range paths {
		if p == strings.TrimSpace(query) {
			best = p
			break
		}
	}
	var candidates []string
	for _, p := range paths {
		if p != best {
			candidates = append(candidates, p)
		}
	}

	out := LibraryExplanation{
		Query:       query,
		Provider:    ProviderInfo{ModulePath: best},
		Candidates:  candidates,
		Limitations: explainLimitations(),
	}

	if lib, found, err := t.s.PrivateLibraryByModule(best); err != nil {
		return LibraryExplanation{}, err
	} else if found {
		out.Provider.DocSynopsis = lib.DocSynopsis
		if repo, commit, ok := t.repoIdentity(lib.IndexID); ok {
			out.Provider.Repo = repo
			out.Provider.Commit = commit
		}
		pkgSeen := map[string]bool{}
		for _, e := range lib.Exports {
			ei := ExportInfo{PackagePath: e.PackagePath, Symbol: e.Symbol, Kind: e.Kind, Doc: e.Doc}
			if node, ok := t.resolveExportNode(lib.IndexID, e); ok {
				ei.Node = node
				ei.Resolved = true
			}
			out.Provider.Exports = append(out.Provider.Exports, ei)
			if e.PackagePath != "" && !pkgSeen[e.PackagePath] {
				pkgSeen[e.PackagePath] = true
				out.Provider.Packages = append(out.Provider.Packages, e.PackagePath)
			}
		}
	}

	cons, err := t.s.PrivateConsumersAcrossRepos(best)
	if err != nil {
		return LibraryExplanation{}, err
	}
	for _, c := range cons {
		if !t.reg.InScope(c.IndexID) {
			continue
		}
		ci := ConsumerInfo{Repo: c.Repo, Commit: c.Commit, Version: c.Version, UsedPackages: c.UsedPackages}
		for _, u := range c.UsedSymbols {
			ul := UsageLink{Usage: u}
			if node, ok := t.resolveUsageNode(c.IndexID, u); ok {
				ul.Node = node
				ul.Resolved = true
			}
			ci.Usages = append(ci.Usages, ul)
		}
		out.Consumers = append(out.Consumers, ci)
	}
	return out, nil
}

// repoIdentity resolves a defining index id to its repo (org/name) and commit.
func (t *Tools) repoIdentity(indexID int64) (string, string, bool) {
	all, err := t.reg.List()
	if err != nil {
		return "", "", false
	}
	for _, ri := range all {
		if ri.IndexID == indexID {
			return ri.Repo, ri.Commit, true
		}
	}
	return "", "", false
}

// resolveExportNode links an export to its provider node, preferring the stored
// NodeID, falling back to a label match.
func (t *Tools) resolveExportNode(indexID int64, e store.PrivateExport) (*store.NodeRow, bool) {
	if e.NodeID != "" {
		if n, found, err := t.s.NodeByID(indexID, e.NodeID); err == nil && found {
			return &n, true
		}
	}
	if e.Symbol != "" {
		if nodes, err := t.s.NodesByLabel(indexID, e.Symbol); err == nil && len(nodes) > 0 {
			n := nodes[0]
			return &n, true
		}
	}
	return nil, false
}

// resolveUsageNode best-effort links a usage to the enclosing consumer node:
// the node in the usage's file with the greatest definition line <= usage line.
func (t *Tools) resolveUsageNode(indexID int64, u store.PrivateUsage) (*store.NodeRow, bool) {
	if u.File == "" {
		return nil, false
	}
	nodes, err := t.s.NodesByFile(indexID, u.File)
	if err != nil || len(nodes) == 0 {
		return nil, false
	}
	bestLine := -1
	var best *store.NodeRow
	for i := range nodes {
		line := parseSourceLine(nodes[i].SourceLocation)
		if line >= 0 && line <= u.Line && line > bestLine {
			bestLine = line
			best = &nodes[i]
		}
	}
	if best == nil {
		return nil, false
	}
	return best, true
}

// parseSourceLine parses a "L<n>" source location into an int, or -1.
func parseSourceLine(loc string) int {
	s := strings.TrimPrefix(strings.TrimSpace(loc), "L")
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}
