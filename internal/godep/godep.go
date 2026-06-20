package godep

import (
	"os"
	"path/filepath"
	"strings"
)

// Dependency is a normalized module dependency.
type Dependency struct {
	ModulePath string
	Version    string
	IsPrivate  bool
	Direct     bool
}

// Export is a normalized exported symbol.
type Export struct {
	PackagePath string
	Symbol      string
	Kind        string // func|constructor|type|interface|const|var
	Doc         string
}

// Library is a provider module's exported API.
type Library struct {
	ModulePath  string
	Readme      string
	DocSynopsis string
	Packages    []string
	Exports     []Export
}

// Usage is a consumer's reference to a private module symbol.
type Usage struct {
	ModulePath  string
	Version     string
	PackagePath string
	Symbol      string
	File        string
	Line        int
}

// Result is the parsed Go data for one repo.
type Result struct {
	ModulePath   string
	Dependencies []Dependency
	// Library: single provider module per Result; in a go.work workspace
	// defining multiple private modules, only the first module's exports are
	// indexed (mergeResults keeps the first). Consumer usages accumulate across
	// all workspace modules. Multi-provider export indexing is a future
	// enhancement.
	Library *Library // set when the repo's own module is private
	Usages  []Usage
}

func isPrivate(modulePath string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(modulePath, p) {
			return true
		}
	}
	return false
}

// Parse reads the given go.mod/go.work files and returns the combined Result.
// privatePrefixes classifies internal modules. Per-file errors are returned as
// warnings; only unexpected failures return a hard error.
func Parse(modules, privatePrefixes []string) (*Result, []string, error) {
	res := &Result{}
	var warns []string
	for _, m := range modules {
		base := filepath.Base(m)
		if base == "go.work" {
			ws, w := parseWork(m, privatePrefixes)
			warns = append(warns, w...)
			mergeResults(res, ws)
			continue
		}
		mr, w := parseModuleDir(filepath.Dir(m), m, privatePrefixes)
		warns = append(warns, w...)
		mergeResults(res, mr)
	}
	return res, warns, nil
}

func mergeResults(dst, src *Result) {
	if src == nil {
		return
	}
	if dst.ModulePath == "" {
		dst.ModulePath = src.ModulePath
	}
	dst.Dependencies = append(dst.Dependencies, src.Dependencies...)
	dst.Usages = append(dst.Usages, src.Usages...)
	if src.Library != nil && dst.Library == nil {
		dst.Library = src.Library
	}
}

func readFile(path string) ([]byte, error) {
	// #nosec G304 -- Go module paths are explicit user manifest inputs.
	return os.ReadFile(path)
}
