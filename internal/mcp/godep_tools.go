package mcp

import "github.com/vend-ai/intel-mcp/internal/store"

const consumersDeferred = "deferred: cross-repo consumer aggregation not available in this change"

// LibraryConsumers is the find_library_consumers result for one repo.
type LibraryConsumers struct {
	ModulePath          string               `json:"module_path"`
	Version             string               `json:"version"`
	UsedPackages        []string             `json:"used_packages"`
	UsedSymbols         []store.PrivateUsage `json:"used_symbols"`
	ConsumedAcrossRepos string               `json:"consumed_across_repos"`
}

// FindPrivateLibrary implements find_private_library: provider libraries plus
// path-only private dependencies matching the query.
func (t *Tools) FindPrivateLibrary(repo, query string) ([]store.PrivateLibraryResult, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	libs, err := t.s.FindPrivateLibraries(ri.IndexID, query)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, l := range libs {
		seen[l.ModulePath] = true
	}
	deps, err := t.s.FindPrivateDeps(ri.IndexID, query)
	if err != nil {
		return nil, err
	}
	for _, d := range deps {
		if !seen[d.ModulePath] {
			libs = append(libs, store.PrivateLibraryResult{ModulePath: d.ModulePath})
			seen[d.ModulePath] = true
		}
	}
	return libs, nil
}

// FindLibraryConsumers implements find_library_consumers (single-repo).
func (t *Tools) FindLibraryConsumers(repo, modulePath string) (LibraryConsumers, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return LibraryConsumers{}, err
	}
	if !ok {
		return LibraryConsumers{}, ErrNotFound
	}
	dep, found, err := t.s.DependencyByModule(ri.IndexID, modulePath)
	if err != nil {
		return LibraryConsumers{}, err
	}
	if !found {
		return LibraryConsumers{}, ErrNotFound
	}
	usages, err := t.s.PrivateUsagesByModule(ri.IndexID, modulePath)
	if err != nil {
		return LibraryConsumers{}, err
	}
	out := LibraryConsumers{
		ModulePath: modulePath, Version: dep.Version,
		UsedSymbols: usages, ConsumedAcrossRepos: consumersDeferred,
	}
	seen := map[string]bool{}
	for _, u := range usages {
		if !seen[u.PackagePath] {
			seen[u.PackagePath] = true
			out.UsedPackages = append(out.UsedPackages, u.PackagePath)
		}
	}
	return out, nil
}
