package godep

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// parseModuleDir parses one module rooted at dir (go.mod at modPath) and returns
// its Result (deps + own module path). Exports/usages are filled by later passes
// (exports.go / usages.go) via parseModuleDir's calls below.
func parseModuleDir(dir, modPath string, privatePrefixes []string) (*Result, []string) {
	var warns []string
	data, err := readFile(modPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", modPath, err)}
	}
	mf, err := modfile.Parse(modPath, data, nil)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", modPath, err)}
	}
	res := &Result{ModulePath: mf.Module.Mod.Path}
	sums := readGoSum(filepath.Join(dir, "go.sum"))
	// replace directives keyed by the original (Old) module path. A local
	// filesystem replacement (=> ../auth) has an empty New.Version.
	replaces := map[string]module.Version{}
	for _, r := range mf.Replace {
		replaces[r.Old.Path] = r.New
	}
	for _, req := range mf.Require {
		d := Dependency{
			ModulePath: req.Mod.Path,
			Version:    req.Mod.Version,
			IsPrivate:  isPrivate(req.Mod.Path, privatePrefixes),
			Direct:     !req.Indirect,
		}
		// Apply a replace targeting this module: reflect the replacement
		// version (empty for a local path replacement). Keep the dependency
		// keyed by the ORIGINAL module path so consumer import resolution and
		// IsPrivate classification (based on the original path) stay correct.
		if newMod, ok := replaces[req.Mod.Path]; ok {
			d.Version = newMod.Version
		}
		if len(sums) > 0 {
			if _, ok := sums[req.Mod.Path+" "+req.Mod.Version]; !ok {
				warns = append(warns, fmt.Sprintf("%s: %s@%s not found in go.sum", modPath, req.Mod.Path, req.Mod.Version))
			}
		}
		res.Dependencies = append(res.Dependencies, d)
	}
	// Provider exports: own module private → extract from dir.
	if isPrivate(res.ModulePath, privatePrefixes) {
		lib, w := extractExports(dir, res.ModulePath)
		warns = append(warns, w...)
		res.Library = lib
	}
	// Consumer usages: imports of private deps in dir source.
	usages, w := extractUsages(dir, res.Dependencies)
	warns = append(warns, w...)
	res.Usages = usages
	return res, warns
}

// parseWork parses a go.work file and merges each used module's Result.
func parseWork(workPath string, privatePrefixes []string) (*Result, []string) {
	data, err := readFile(workPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", workPath, err)}
	}
	wf, err := modfile.ParseWork(workPath, data, nil)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", workPath, err)}
	}
	res := &Result{}
	var warns []string
	workDir := filepath.Dir(workPath)
	for _, u := range wf.Use {
		modDir := filepath.Join(workDir, filepath.FromSlash(u.Path))
		mr, w := parseModuleDir(modDir, filepath.Join(modDir, "go.mod"), privatePrefixes)
		warns = append(warns, w...)
		mergeResults(res, mr)
	}
	return res, warns
}

// readGoSum returns a set of "module version" present in a go.sum (empty if absent).
func readGoSum(path string) map[string]struct{} {
	// #nosec G304 -- go.sum path is derived from explicit user manifest module paths.
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	out := map[string]struct{}{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 {
			ver := strings.TrimSuffix(fields[1], "/go.mod")
			out[fields[0]+" "+ver] = struct{}{}
		}
	}
	return out
}
